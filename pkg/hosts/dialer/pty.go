//go:build darwin || linux
// +build darwin linux

package dialer

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/creack/pty"
)

var _ hosts.Shell = &PtyShell{}

// PtyShell struct for pty dialer.
type PtyShell struct {
	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer
	Writer io.Writer

	ctx  context.Context
	conn *os.File
	cmd  *exec.Cmd
}

// NewPtyDialer returns new pty dialer struct.
func NewPtyShell(cmd *exec.Cmd) (*PtyShell, error) {
	if cmd == nil {
		return nil, errors.New("[pty-dialer] no cmd is specified")
	}

	return &PtyShell{ctx: context.Background(), cmd: cmd}, nil
}

// Close close the pty connection.
func (d *PtyShell) Close() error {
	if d.conn != nil {
		if err := d.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SetIO set dialer's reader and writer.
func (d *PtyShell) SetIO(stdout, stderr io.Writer, stdin io.ReadCloser) {
	d.Stdout = stdout
	d.Stderr = stderr
	d.Stdin = stdin
}

func (d *PtyShell) Terminal() error {
	return errors.New("pty terminal not supported")
}

// OpenTerminal open pty websocket terminal.
func (d *PtyShell) OpenTerminal(win hosts.ShellWindowSize) error {
	p, err := pty.StartWithSize(d.cmd, &pty.Winsize{
		Rows: uint16(win.Height),
		Cols: uint16(win.Width),
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
func (d *PtyShell) ChangeWindowSize(win hosts.ShellWindowSize) error {
	return pty.Setsize(d.conn, &pty.Winsize{
		Rows: uint16(win.Height),
		Cols: uint16(win.Width),
	})
}

// Wait waits for the command to exit.
func (d *PtyShell) Wait() error {
	return d.cmd.Wait()
}

func (d *PtyShell) Write(b []byte) error {
	_, err := d.conn.Write(b)
	return err
}
