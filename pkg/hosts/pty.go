// +build darwin linux

package hosts

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type PtyDialer struct {
	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer
	Writer io.Writer

	Height int
	Weight int

	ctx  context.Context
	conn *os.File
	cmd  *exec.Cmd
}

func NewPtyDialer(cmd *exec.Cmd) (*PtyDialer, error) {
	if cmd == nil {
		return nil, errors.New("[pty-dialer] no cmd is specified")
	}

	return &PtyDialer{ctx: context.Background(), cmd: cmd}, nil
}

// Close close the pty connection.
func (d *PtyDialer) Close() error {
	if d.conn != nil {
		if err := d.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SetIO set dialer's reader and writer.
func (d *PtyDialer) SetIO(stdout, stderr io.Writer, stdin io.ReadCloser) {
	d.Stdout = stdout
	d.Stderr = stderr
	d.Stdin = stdin
}

// SetWindowSize set tty window size size.
func (d *PtyDialer) SetWindowSize(height, weight int) {
	d.Height = height
	d.Weight = weight
}

// OpenTerminal open pty websocket terminal.
func (d *PtyDialer) OpenTerminal() error {
	p, err := pty.StartWithSize(d.cmd, &pty.Winsize{
		Rows: uint16(d.Height),
		Cols: uint16(d.Weight),
	})
	if err != nil {
		return err
	}

	d.conn = p

	go func() {
		_, _ = io.Copy(d.conn, d.Stdin)
	}()

	go func() {
		_, _ = io.Copy(d.Stderr, d.conn)
	}()

	return nil
}

// ChangeWindowSize changes to the current window size.
func (d *PtyDialer) ChangeWindowSize(win *WindowSize) error {
	return pty.Setsize(d.conn, &pty.Winsize{
		Rows: uint16(win.Height),
		Cols: uint16(win.Width),
	})
}

// Wait waits for the command to exit.
func (d *PtyDialer) Wait() error {
	return d.cmd.Wait()
}

func (d *PtyDialer) Write(b []byte) error {
	_, err := d.conn.Write(b)
	return err
}
