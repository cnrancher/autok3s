//go:build windows
// +build windows

package hosts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// PtyDialer struct for pty dialer.
type PtyDialer struct {
	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer
	Writer io.Writer

	ctx  context.Context
	conn *os.File
	cmd  *exec.Cmd

	err error
}

// NewPtyDialer returns new pty dialer struct.
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

// SetStdio set dialer's reader and writer.
func (d *PtyDialer) SetStdio(stdout, stderr io.Writer, stdin io.ReadCloser) *PtyDialer {
	d.Stdout = stdout
	d.Stderr = stderr
	d.Stdin = stdin
	return d
}

// WebSocketTerminal open pty websocket terminal.
func (d *PtyDialer) WebSocketTerminal() error {
	return fmt.Errorf("not support windows")
}

// ChangeSize changes to the current win size.
func (d *PtyDialer) ChangeSize(win WindowSize) error {
	return nil
}
