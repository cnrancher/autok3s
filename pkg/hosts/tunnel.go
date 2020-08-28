package hosts

import (
	"golang.org/x/crypto/ssh"
)

type Tunnel struct {
	conn *ssh.Client
}

func (t *Tunnel) Close() error {
	return t.conn.Close()
}

func (t *Tunnel) ExecuteCommand(cmd string) (string, error) {
	session, err := t.conn.NewSession()
	if err != nil {
		return "", err
	}

	defer func() {
		_ = session.Close()
	}()

	b, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
