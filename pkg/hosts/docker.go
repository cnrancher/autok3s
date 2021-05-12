package hosts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/ioutils"
	dockerSig "github.com/docker/docker/pkg/signal"
	"github.com/moby/term"
	dockerutils "github.com/rancher/k3d/v4/pkg/runtimes/docker"
	k3dtypes "github.com/rancher/k3d/v4/pkg/types"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

// the default escape key sequence: ctrl-p, ctrl-q.
var defaultEscapeKeys = []byte{16, 17}

type DockerDialer struct {
	execID string

	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer
	Writer io.Writer

	Height int
	Weight int

	ctx      context.Context
	client   *client.Client
	response *dockertypes.HijackedResponse
}

func NewDockerDialer(n *types.Node) (*DockerDialer, error) {
	if n.InstanceID == "" {
		return nil, errors.New("[docker-dialer] no container ID is specified")
	}

	d := &DockerDialer{
		ctx: context.Background(),
	}

	// create docker client.
	docker, err := dockerutils.GetDockerClient()
	if err != nil {
		return nil, fmt.Errorf("[docker-dialer] failed to get docker client: %w", err)
	}

	d.client = docker

	// (1) list containers which have the default k3d labels attached.
	f := filters.NewArgs()

	// regex filtering for exact name match.
	// Assumptions:
	// -> container names start with a / (see https://github.com/moby/moby/issues/29997).
	// -> user input may or may not have the "k3d-" prefix.
	f.Add("name", fmt.Sprintf("^/?(%s-)?%s$", k3dtypes.DefaultObjectNamePrefix, n.InstanceID))

	containers, err := docker.ContainerList(d.ctx, dockertypes.ContainerListOptions{
		Filters: f,
		All:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("[docker-dialer] failed to list containers: %+v", err)
	}

	if len(containers) > 1 {
		return nil, fmt.Errorf("[docker-dialer] failed to get a single container for name '%s'. found: %d", n.InstanceID, len(containers))
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("[docker-dialer] didn't find container for node '%s'", n.InstanceID)
	}

	container := containers[0]

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		// create docker container exec.
		exec, err := docker.ContainerExecCreate(d.ctx, container.ID, dockertypes.ExecConfig{
			Privileged:   true,
			Tty:          true,
			AttachStdin:  true,
			AttachStderr: true,
			AttachStdout: true,
			Cmd:          []string{"/bin/sh"},
		})
		if err != nil {
			return false, err
		}

		// attaches a connection to an exec process in the server.
		execAttach, err := docker.ContainerExecAttach(d.ctx, exec.ID, dockertypes.ExecStartCheck{
			Tty: true,
		})
		if err != nil {
			return false, err
		}

		d.execID = exec.ID
		d.response = &execAttach

		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("[docker-dialer] establish docker exec [%s] error: %w", n.InstanceID, err)
	}

	return d, nil
}

// Close close the Docker connection.
func (d *DockerDialer) Close() error {
	if d.response != nil {
		d.response.Close()
	}
	if d.client != nil {
		if err := d.client.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SetStdio set dialer's reader and writer.
func (d *DockerDialer) SetStdio(stdout, stderr io.Writer, stdin io.ReadCloser) *DockerDialer {
	d.SetIO(stdout, stderr, stdin)
	return d
}

// SetIO set dialer's reader and writer.
func (d *DockerDialer) SetIO(stdout, stderr io.Writer, stdin io.ReadCloser) {
	d.Stdout = stdout
	d.Stderr = stderr
	d.Stdin = stdin
}

// SetWindowSize set dialer's default win size.
func (d *DockerDialer) SetWindowSize(height, weight int) {
	d.Height = height
	d.Weight = weight
}

// SetWriter set dialer's logs writer.
func (d *DockerDialer) SetWriter(w io.Writer) *DockerDialer {
	d.Writer = w
	return d
}

// Terminal open docker exec terminal.
func (d *DockerDialer) Terminal() error {
	defer func() {
		_ = d.Close()
	}()

	if err := d.ExecStart(true); err != nil {
		return err
	}

	fd, _ := term.GetFdInfo(d.Stderr)
	if term.IsTerminal(fd) {
		if err := d.MonitorTtySize(d.ctx); err != nil {
			logrus.Errorf("[docker-dialer] error monitoring tty size: %s", err.Error())
		}
	}

	if err := d.Wait(); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
	}

	return nil
}

// WebSocketTerminal open docker websocket terminal.
func (d *DockerDialer) OpenTerminal() error {
	return d.ExecStart(false)
}

// ExecStart handles setting up the IO and then begins streaming stdin/stdout
// to/from the hijacked connection, blocking until it is either done reading
// output, the user inputs the detach key sequence when in TTY mode, or when
// the given context is cancelled.
// Borrowed from https://github.com/docker/cli/blob/master/cli/command/container/hijack.go#L40.
func (d *DockerDialer) ExecStart(needRestore bool) error {
	restoreInput, err := d.setInput(needRestore)
	if err != nil {
		return fmt.Errorf("[docker-dialer] unable to setup input stream: %s", err)
	}

	defer restoreInput()

	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		errCh <- func() error {
			outputDone := d.beginOutputStream(restoreInput)
			inputDone, detached := d.beginInputStream(restoreInput)

			select {
			case err := <-outputDone:
				return err
			case <-inputDone:
				// input stream has closed.
				if d.Stdout != nil || d.Stderr != nil {
					// wait for output to complete streaming.
					select {
					case err := <-outputDone:
						return err
					case <-d.ctx.Done():
						return d.ctx.Err()
					}
				}
				return nil
			case err := <-detached:
				// got a detach key sequence.
				return err
			case <-d.ctx.Done():
				return d.ctx.Err()
			}
		}()
	}()

	if err := <-errCh; err != nil {
		logrus.Errorf("[docker-dialer] error hijack: %s", err)
		return err
	}

	return nil
}

// Borrowed from https://github.com/docker/cli/blob/master/cli/command/container/exec.go#L180.
func (d *DockerDialer) Wait() error {
	resp, err := d.client.ContainerExecInspect(d.ctx, d.execID)
	if err != nil {
		// if we can't connect, then the daemon probably died.
		if !client.IsErrConnectionFailed(err) {
			return err
		}
		return io.ErrUnexpectedEOF
	}
	status := resp.ExitCode
	if status != 0 {
		return io.EOF
	}
	return nil
}

// ResizeTtyTo changes the size of the tty for an exec process running inside a container.
func (d *DockerDialer) ResizeTtyTo(ctx context.Context, height, width uint) error {
	if height == 0 && width == 0 {
		return nil
	}

	return d.client.ContainerExecResize(ctx, d.execID, dockertypes.ResizeOptions{
		Height: height,
		Width:  width,
	})
}

// ResizeTty changes to the current win size.
func (d *DockerDialer) ResizeTty(ctx context.Context) error {
	fd, _ := term.GetFdInfo(d.Stdout)
	winSize, err := term.GetWinsize(fd)
	if err != nil {
		return err
	}

	return d.ResizeTtyTo(ctx, uint(winSize.Height), uint(winSize.Width))
}

// MonitorTtySize monitor and change tty size.
// Borrowed from https://github.com/docker/cli/blob/master/cli/command/container/tty.go#L71.
func (d *DockerDialer) MonitorTtySize(ctx context.Context) error {
	ttyFunc := d.ResizeTty

	if err := ttyFunc(ctx); err != nil {
		go func() {
			var err error
			for retry := 0; retry < 5; retry++ {
				time.Sleep(10 * time.Millisecond)
				if err = ttyFunc(ctx); err == nil {
					break
				}
			}
			if err != nil {
				logrus.Errorf("[docker-dialer] failed to resize tty, using default size")
			}
		}()
	}

	if runtime.GOOS == "windows" {
		go func() {
			fd, _ := term.GetFdInfo(d.Stdout)
			prevWinSize, err := term.GetWinsize(fd)
			if err != nil {
				return
			}
			prevH := prevWinSize.Height
			prevW := prevWinSize.Width
			for {
				time.Sleep(time.Millisecond * 250)
				winSize, err := term.GetWinsize(fd)
				if err != nil {
					return
				}
				h := winSize.Height
				w := winSize.Width
				if prevW != w || prevH != h {
					_ = d.ResizeTty(ctx)
				}
				prevH = h
				prevW = w
			}
		}()
	} else {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, dockerSig.SIGWINCH)
		go func() {
			for range sigChan {
				_ = d.ResizeTty(ctx)
			}
		}()
	}
	return nil
}

// Borrowed from https://github.com/docker/cli/blob/master/cli/command/container/hijack.go#L74.
func (d *DockerDialer) setInput(needRestore bool) (restore func(), err error) {
	if d.Stdin == nil {
		// no need to setup input TTY.
		// the restore func is a nop.
		return func() {}, nil
	}

	// use sync.Once so we may call restore multiple times but ensure we
	// only restore the terminal once.
	var restoreOnce sync.Once

	if needRestore {
		inFd, _ := term.GetFdInfo(d.Stdin)
		outFd, _ := term.GetFdInfo(d.Stderr)
		inState, _ := term.SetRawTerminal(inFd)
		outState, _ := term.SetRawTerminalOutput(outFd)

		restore = func() {
			restoreOnce.Do(func() {
				if outState != nil {
					_ = term.RestoreTerminal(inFd, outState)
				}
				if inState != nil {
					_ = term.RestoreTerminal(outFd, inState)
				}
			})
		}
	} else {
		restore = func() {}
	}

	// wrap the input to detect detach escape sequence.
	// use default escape keys if an invalid sequence is given.
	escapeKeys := defaultEscapeKeys
	d.Stdin = ioutils.NewReadCloserWrapper(term.NewEscapeProxy(d.Stdin, escapeKeys), d.Stdin.Close)

	return restore, nil
}

// Borrowed from https://github.com/docker/cli/blob/master/cli/command/container/hijack.go#L111.
func (d *DockerDialer) beginOutputStream(restoreInput func()) <-chan error {
	outputDone := make(chan error)
	go func() {
		var err error

		// when TTY is ON, use regular copy.
		_, err = io.Copy(d.Stderr, d.response.Reader)
		// we should restore the terminal as soon as possible,
		// once the connection ends so any following print,
		// messages will be in normal type.

		restoreInput()

		if err != nil {
			logrus.Errorf("[docker-dialer] error receive Stdout: %s", err)
		}

		outputDone <- err
	}()

	return outputDone
}

// Borrowed from https://github.com/docker/cli/blob/master/cli/command/container/hijack.go#L144.
func (d *DockerDialer) beginInputStream(restoreInput func()) (doneC <-chan struct{}, detachedC <-chan error) {
	inputDone := make(chan struct{})
	detached := make(chan error)

	go func() {
		if d.Stdin != nil {
			_, err := io.Copy(d.response.Conn, d.Stdin)
			// we should restore the terminal as soon as possible,
			// once the connection ends so any following print,
			// messages will be in normal type.
			restoreInput()

			if _, ok := err.(term.EscapeError); ok {
				detached <- err
				return
			}

			if err != nil {
				// this error will also occur on the receive,
				// side (from stdout) where it will be,
				// propagated back to the caller.
				logrus.Debugf("[docker-dialer] error send Stdin: %s", err)
			}
		}

		if err := d.response.CloseWrite(); err != nil {
			logrus.Debugf("[docker-dialer] couldn't send EOF: %s", err)
		}

		close(inputDone)
	}()

	return inputDone, detached
}

// ChangeWindowSize change tty window size for websocket.
func (d *DockerDialer) ChangeWindowSize(win *WindowSize) error {
	return d.ResizeTtyTo(d.ctx, uint(win.Height), uint(win.Width))
}

func (d *DockerDialer) Write(b []byte) error {
	return nil
}
