package hosts

import (
	"bytes"
	"io"

	"golang.org/x/crypto/ssh"
)

type Tunnel struct {
	Stdout io.Writer
	Stderr io.Writer

	err  error
	conn *ssh.Client
	cmd  *bytes.Buffer
}

func (t *Tunnel) Close() error {
	return t.conn.Close()
}

func (t *Tunnel) Cmd(cmd string) *Tunnel {
	if t.cmd == nil {
		t.cmd = bytes.NewBufferString(cmd + "\n")
	}

	_, err := t.cmd.WriteString(cmd + "\n")
	if err != nil {
		t.err = err
	}

	return t
}

func (t *Tunnel) Run() error {
	if t.err != nil {
		return t.err
	}

	return t.executeCommands()
}

func (t *Tunnel) SetStdio(stdout, stderr io.Writer) *Tunnel {
	t.Stdout = stdout
	t.Stderr = stderr
	return t
}

func (t *Tunnel) executeCommands() error {
	for {
		cmd, err := t.cmd.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := t.executeCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (t *Tunnel) executeCommand(cmd string) error {
	session, err := t.conn.NewSession()
	if err != nil {
		return err
	}

	defer func() {
		_ = session.Close()
	}()

	session.Stdout = t.Stdout
	session.Stderr = t.Stderr

	if err := session.Run(cmd); err != nil {
		return err
	}

	return nil
}
