package hosts

import (
	"bytes"
	"io"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type Tunnel struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Writer io.Writer
	Modes  ssh.TerminalModes
	Term   string
	Height int
	Weight int

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
		return t
	}

	_, err := t.cmd.WriteString(cmd + "\n")
	if err != nil {
		t.err = err
	}

	return t
}

func (t *Tunnel) Terminal() error {
	session, err := t.conn.NewSession()
	defer func() {
		_ = session.Close()
	}()
	if err != nil {
		return err
	}

	term := os.Getenv("TERM")
	if term == "" {
		t.Term = "xterm-256color"
	}
	t.Modes = ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	fd := int(os.Stdin.Fd())
	oldState, err := terminal.MakeRaw(fd)
	defer func() {
		_ = terminal.Restore(fd, oldState)
	}()
	if err != nil {
		return err
	}

	t.Weight, t.Height, err = terminal.GetSize(fd)
	if err != nil {
		return err
	}

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.RequestPty(t.Term, t.Height, t.Weight, t.Modes); err != nil {
		return err
	}

	if err := session.Shell(); err != nil {
		return err
	}

	if err := session.Wait(); err != nil {
		return err
	}

	return nil
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

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return err
	}

	var outWriter, errWriter io.Writer
	if t.Writer != nil {
		outWriter = io.MultiWriter(t.Stdout, t.Writer)
		errWriter = io.MultiWriter(t.Stderr, t.Writer)
	} else {
		outWriter = io.MultiWriter(os.Stdout, t.Stdout)
		errWriter = io.MultiWriter(os.Stderr, t.Stderr)
	}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		io.Copy(outWriter, stdoutPipe)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		io.Copy(errWriter, stderrPipe)
		wg.Done()
	}()

	err = session.Run(cmd)

	wg.Wait()

	return err
}

func (t *Tunnel) Session() (*ssh.Session, error) {
	return t.conn.NewSession()
}
