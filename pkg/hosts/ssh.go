package hosts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/moby/term"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/apimachinery/pkg/util/wait"
)

type SSHDialer struct {
	sshKey          string
	sshCert         string
	sshAddress      string
	username        string
	password        string
	passphrase      string
	useSSHAgentAuth bool

	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer
	Writer io.Writer

	Height int
	Weight int

	Term  string
	Modes ssh.TerminalModes

	ctx  context.Context
	conn *ssh.Client
	cmd  *bytes.Buffer

	err error
}

func NewSSHDialer(n *types.Node, timeout bool) (*SSHDialer, error) {
	if len(n.PublicIPAddress) <= 0 && n.InstanceID == "" {
		return nil, errors.New("[ssh-dialer] no node IP or node ID is specified")
	}

	d := &SSHDialer{
		username:        n.SSHUser,
		password:        n.SSHPassword,
		passphrase:      n.SSHKeyPassphrase,
		useSSHAgentAuth: n.SSHAgentAuth,
		sshCert:         n.SSHCert,
		ctx:             context.Background(),
	}

	// IP addresses are preferred.
	if len(n.PublicIPAddress) > 0 {
		d.sshAddress = fmt.Sprintf("%s:%s", n.PublicIPAddress[0], n.SSHPort)
	} else {
		d.sshAddress = n.InstanceID
	}

	if d.password == "" && d.sshKey == "" && !d.useSSHAgentAuth && len(n.SSHKeyPath) > 0 {
		var err error
		d.sshKey, err = utils.SSHPrivateKeyPath(n.SSHKeyPath)
		if err != nil {
			return nil, err
		}

		if d.sshCert == "" && len(n.SSHCertPath) > 0 {
			d.sshCert, err = utils.SSHCertificatePath(n.SSHCertPath)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		c, err := d.Dial(timeout)
		if err != nil {
			return false, nil
		}

		d.conn = c

		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("[ssh-dialer] init dialer [%s] error: %w", d.sshAddress, err)
	}

	return d, nil
}

// Dial handshake with ssh address.
func (d *SSHDialer) Dial(t bool) (*ssh.Client, error) {
	timeout := time.Duration((common.Backoff.Steps - 1) * int(common.Backoff.Duration))
	if !t {
		timeout = 0
	}

	cfg, err := utils.GetSSHConfig(d.username, d.sshKey, d.passphrase, d.sshCert, d.password, timeout, d.useSSHAgentAuth)
	if err != nil {
		return nil, err
	}
	// establish connection with SSH server.
	return ssh.Dial("tcp", d.sshAddress, cfg)
}

// Close close the SSH connection.
func (d *SSHDialer) Close() error {
	if d.conn != nil {
		if err := d.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SetStdio set dialer's reader and writer.
func (d *SSHDialer) SetStdio(stdout, stderr io.Writer, stdin io.ReadCloser) *SSHDialer {
	d.Stdout = stdout
	d.Stderr = stderr
	d.Stdin = stdin
	return d
}

// SetDefaultSize set dialer's default win size.
func (d *SSHDialer) SetDefaultSize(height, weight int) *SSHDialer {
	d.Height = height
	d.Weight = weight
	return d
}

// SetWriter set dialer's logs writer.
func (d *SSHDialer) SetWriter(w io.Writer) *SSHDialer {
	d.Writer = w
	return d
}

// Cmd pass commands in dialer, support multiple calls, e.g. d.Cmd("ls").Cmd("id").
func (d *SSHDialer) Cmd(cmd string) *SSHDialer {
	if d.cmd == nil {
		d.cmd = bytes.NewBufferString(cmd + "\n")
		return d
	}

	_, err := d.cmd.WriteString(cmd + "\n")
	if err != nil {
		d.err = err
	}

	return d
}

// Run run commands in remote server via SSH tunnel.
func (d *SSHDialer) Run() error {
	if d.err != nil {
		return d.err
	}

	return d.executeCommands()
}

// Terminal open ssh terminal.
func (d *SSHDialer) Terminal() error {
	session, err := d.conn.NewSession()
	defer func() {
		_ = session.Close()
	}()
	if err != nil {
		return err
	}

	d.Term = os.Getenv("TERM")
	if d.Term == "" {
		d.Term = "xterm"
	}
	d.Modes = ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	fdInfo, _ := term.GetFdInfo(d.Stdout)
	fd := int(fdInfo)

	oldState, err := terminal.MakeRaw(fd)
	defer func() {
		_ = terminal.Restore(fd, oldState)
	}()
	if err != nil {
		return err
	}

	d.Weight, d.Height, err = terminal.GetSize(fd)
	if err != nil {
		return err
	}

	session.Stdin = d.Stdin
	session.Stdout = d.Stdout
	session.Stderr = d.Stderr

	if err := session.RequestPty(d.Term, d.Height, d.Weight, d.Modes); err != nil {
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

// WebSocketTerminal open websocket terminal.
func (d *SSHDialer) WebSocketTerminal(session *ssh.Session) error {
	d.Term = os.Getenv("TERM")
	if d.Term == "" {
		d.Term = "xterm"
	}
	d.Modes = ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	session.Stdin = d.Stdin
	session.Stdout = d.Stdout
	session.Stderr = d.Stderr

	if err := session.RequestPty(d.Term, d.Height, d.Weight, d.Modes); err != nil {
		return err
	}

	if err := session.Shell(); err != nil {
		return err
	}

	return nil
}

func (d *SSHDialer) executeCommands() error {
	for {
		cmd, err := d.cmd.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := d.executeCommand(cmd); err != nil {
			return err
		}
	}

	return nil
}

func (d *SSHDialer) executeCommand(cmd string) error {
	session, err := d.conn.NewSession()
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
	if d.Writer != nil {
		outWriter = io.MultiWriter(d.Stdout, d.Writer)
		errWriter = io.MultiWriter(d.Stderr, d.Writer)
	} else {
		outWriter = io.MultiWriter(os.Stdout, d.Stdout)
		errWriter = io.MultiWriter(os.Stderr, d.Stderr)
	}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		_, _ = io.Copy(outWriter, stdoutPipe)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		_, _ = io.Copy(errWriter, stderrPipe)
		wg.Done()
	}()

	err = session.Run(cmd)

	wg.Wait()

	return err
}
