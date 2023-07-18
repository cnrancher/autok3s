package hosts

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/sirupsen/logrus"

	"github.com/moby/term"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/apimachinery/pkg/util/wait"
)

var defaultBackoff = wait.Backoff{
	Duration: 15 * time.Second,
	Factor:   1,
	Steps:    5,
}

const scriptWrapper = `#!/bin/sh
set -e
[ -n "%s" ] && set -x || true
%s
`

// SSHDialer struct for ssh dialer.
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

	Term  string
	Modes ssh.TerminalModes

	ctx     context.Context
	conn    *ssh.Client
	session *ssh.Session
	cmd     *bytes.Buffer

	err error

	uid    int
	logger *logrus.Logger
}

// NewSSHDialer returns new ssh dialer.
func NewSSHDialer(n *types.Node, timeout bool, logger *logrus.Logger) (*SSHDialer, error) {
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
		logger:          logger,
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

	try := 0
	if err := wait.ExponentialBackoff(defaultBackoff, func() (bool, error) {
		try++
		logger.Infof("the %d/%d time tring to ssh to %s with user %s", try, defaultBackoff.Steps, d.sshAddress, d.username)
		c, err := d.Dial(timeout)
		if err != nil {
			return false, nil
		}

		d.conn = c

		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("[ssh-dialer] init dialer [%s] error: %w", d.sshAddress, err)
	}

	session, err := d.conn.NewSession()
	if err != nil {
		return nil, err
	}
	d.session = session

	return d, d.getUserID()
}

// Dial handshake with ssh address.
func (d *SSHDialer) Dial(t bool) (*ssh.Client, error) {
	timeout := defaultBackoff.Duration
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

// Wait waits for the remote command to exit.
func (d *SSHDialer) Wait() error {
	if d.session != nil {
		return d.session.Wait()
	}
	return nil
}

// Close close the SSH connection.
func (d *SSHDialer) Close() error {
	if d.session != nil {
		if err := d.session.Close(); err != nil {
			return err
		}
	}
	if d.conn != nil {
		if err := d.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SetStdio set dialer's reader and writer.
func (d *SSHDialer) SetStdio(stdout, stderr io.Writer, stdin io.ReadCloser) *SSHDialer {
	d.SetIO(stdout, stderr, stdin)
	return d
}

// SetIO set dialer's reader and writer.
func (d *SSHDialer) SetIO(stdout, stderr io.Writer, stdin io.ReadCloser) {
	d.Stdout = stdout
	d.Stderr = stderr
	d.Stdin = stdin
}

// ChangeWindowSize change the window size for current session.
func (d *SSHDialer) ChangeWindowSize(win WindowSize) error {
	return d.session.WindowChange(win.Height, win.Width)
}

// SetWriter set dialer's logs writer.
func (d *SSHDialer) SetWriter(w io.Writer) *SSHDialer {
	d.Writer = w
	return d
}

// Cmd pass commands in dialer, support multiple calls, e.g. d.Cmd("ls").Cmd("id").
func (d *SSHDialer) Cmd(cmds ...string) *SSHDialer {
	for _, cmd := range cmds {
		if d.cmd == nil {
			d.cmd = bytes.NewBuffer([]byte{})
		}

		_, err := d.cmd.WriteString(cmd + "\n")
		if err != nil {
			d.err = err
		}
	}

	return d
}

// Run commands in remote server via SSH tunnel.
func (d *SSHDialer) Run() error {
	if d.err != nil {
		return d.err
	}

	return d.executeCommands()
}

// Terminal starts a login shell on the remote host for CLI.
func (d *SSHDialer) Terminal() error {
	var win WindowSize
	fdInfo, _ := term.GetFdInfo(d.Stdout)
	fd := int(fdInfo)

	oldState, err := terminal.MakeRaw(fd)
	defer func() {
		_ = terminal.Restore(fd, oldState)
	}()
	if err != nil {
		return err
	}

	win.Width, win.Height, err = terminal.GetSize(fd)
	if err != nil {
		return err
	}

	if err := d.OpenTerminal(win); err != nil {
		return err
	}

	return d.session.Wait()
}

// OpenTerminal starts a login shell on the remote host.
func (d *SSHDialer) OpenTerminal(win WindowSize) error {
	d.Term = os.Getenv("TERM")
	if d.Term == "" {
		d.Term = "xterm"
	}
	d.Modes = ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	d.session.Stdin = d.Stdin
	d.session.Stdout = d.Stdout
	d.session.Stderr = d.Stderr

	if err := d.session.RequestPty(d.Term, win.Height, win.Width, d.Modes); err != nil {
		return err
	}

	return d.session.Shell()
}

func (d *SSHDialer) executeCommands() error {
	encodedCMD := d.wrapCommands(d.cmd.String())
	cmd := fmt.Sprintf("echo \"%s\" | base64 -d | %s sh -", encodedCMD, d.shouldRoot())
	d.logger.Debugf("executing cmd: %s", cmd)
	return d.executeCommand(cmd)
}

func (d *SSHDialer) executeCommand(cmd string) error {
	stdoutPipe, err := d.session.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := d.session.StderrPipe()
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

	err = d.session.Run(cmd)

	wg.Wait()

	return err
}

func (d *SSHDialer) Write(b []byte) error {
	return nil
}

func (d *SSHDialer) GetClient() *ssh.Client {
	return d.conn
}

func (d *SSHDialer) getUserID() error {
	session, err := d.conn.NewSession()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	output, err := session.Output("id -u")
	if err != nil {
		return fmt.Errorf("failed to get current user id from remote host %s, %v", d.sshAddress, err)
	}
	// it should return a number with user id if ok
	d.uid, err = strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return fmt.Errorf("failed to parse uid output from remote host, output: %s, %v", string(output), err)
	}
	return nil
}

func (d *SSHDialer) shouldRoot() string {
	if d.uid == 0 {
		return ""
	}
	return "sudo"
}

func (d *SSHDialer) wrapCommands(cmd string) string {
	// set the beginning debug flag
	var debug string
	if d.logger.Level == logrus.DebugLevel {
		debug = "true"
	}
	arg := []any{debug, cmd}
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(scriptWrapper, arg...)))
}

func (d *SSHDialer) RenewSession() error {
	_ = d.session.Close()
	var err error
	d.session, err = d.conn.NewSession()
	if d.cmd != nil {
		d.cmd.Reset()
	}

	return err
}
