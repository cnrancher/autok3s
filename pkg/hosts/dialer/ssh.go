package dialer

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/sirupsen/logrus"

	"github.com/moby/term"
	"golang.org/x/crypto/ssh"
	xterm "golang.org/x/term"
	"k8s.io/apimachinery/pkg/util/wait"
)

var defaultBackoff = wait.Backoff{
	Duration: 15 * time.Second,
	Factor:   1,
	Steps:    5,
}

const scriptWrapper = `#!/bin/sh
set -e
%s
`

var _ hosts.Shell = &SSHShell{}
var _ hosts.Script = &SSHDialer{}

// SSHDialer struct for ssh dialer.
type SSHDialer struct {
	sshKey          string
	sshCert         string
	sshAddress      string
	username        string
	password        string
	passphrase      string
	useSSHAgentAuth bool

	conn *ssh.Client

	uid    int
	logger *logrus.Logger

	shells map[hosts.Shell]hosts.Shell
}

// NewSSHDialer returns new ssh dialer.
func NewSSHDialer(n *types.Node, timeout bool, logger *logrus.Logger) (*SSHDialer, error) {
	if len(n.PublicIPAddress) <= 0 && n.InstanceID == "" {
		return nil, errors.New("[ssh-dialer] no node IP or node ID is specified")
	}
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	d := &SSHDialer{
		username:        n.SSHUser,
		password:        n.SSHPassword,
		passphrase:      n.SSHKeyPassphrase,
		useSSHAgentAuth: n.SSHAgentAuth,
		sshCert:         n.SSHCert,
		logger:          logger,
		shells:          map[hosts.Shell]hosts.Shell{},
		uid:             -1,
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

	return d, nil
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

func (d *SSHDialer) GetClient() *ssh.Client {
	return d.conn
}

func (d *SSHDialer) getUserID() error {
	if d.uid >= 0 {
		return nil
	}
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

func (d *SSHDialer) Close() error {
	if d.conn != nil {
		return d.conn.Close()
	}
	for _, shell := range d.shells {
		shell.Close()
		delete(d.shells, shell)
	}
	return nil
}

func (d *SSHDialer) wrapCommands(cmd string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(scriptWrapper, cmd)))
}

func (d *SSHDialer) ExecuteCommands(cmds ...string) (string, error) {
	if err := d.getUserID(); err != nil {
		return "", err
	}

	sudo := ""
	if d.uid > 0 {
		sudo = "sudo"
	}

	session, err := d.conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	encodedCMD := d.wrapCommands(strings.Join(cmds, "\n"))
	cmd := fmt.Sprintf("echo \"%s\" | base64 -d | %s sh -", encodedCMD, sudo)
	d.logger.Debugf("executing cmd: %s", cmd)

	output := bytes.NewBuffer([]byte{})
	combinedOutput := singleWriter{
		b: io.MultiWriter(d.logger.Out, output),
	}
	session.Stderr = &combinedOutput
	session.Stdout = &combinedOutput
	err = session.Run(cmd)

	return output.String(), err
}

func (d *SSHDialer) OpenShell() (hosts.Shell, error) {
	shell := &SSHShell{
		dialer: d,
	}
	session, err := d.conn.NewSession()
	if err != nil {
		return nil, err
	}
	shell.session = session
	d.shells[shell] = shell
	return shell, nil
}

type SSHShell struct {
	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer
	Writer io.Writer

	Term    string
	Modes   ssh.TerminalModes
	session *ssh.Session
	dialer  *SSHDialer
}

func (d *SSHShell) Write(_ []byte) error {
	return nil
}

// Wait waits for the remote command to exit.
func (d *SSHShell) Wait() error {
	if d.session != nil {
		return d.session.Wait()
	}
	return nil
}

// Close close the SSH connection.
func (d *SSHShell) Close() error {
	defer delete(d.dialer.shells, d)
	if d.session != nil {
		if err := d.session.Close(); err != nil {
			return err
		}
	}
	return nil
}

// SetIO set dialer's reader and writer.
func (d *SSHShell) SetIO(stdout, stderr io.Writer, stdin io.ReadCloser) {
	d.Stdout = stdout
	d.Stderr = stderr
	d.Stdin = stdin
}

// ChangeWindowSize change the window size for current session.
func (d *SSHShell) ChangeWindowSize(win hosts.ShellWindowSize) error {
	return d.session.WindowChange(win.Height, win.Width)
}

// SetWriter set dialer's logs writer.
func (d *SSHShell) SetWriter(w io.Writer) *SSHShell {
	d.Writer = w
	return d
}

// Terminal starts a login shell on the remote host for CLI.
func (d *SSHShell) Terminal() error {
	var win hosts.ShellWindowSize
	fdInfo, _ := term.GetFdInfo(d.Stdout)
	fd := int(fdInfo)

	oldState, err := xterm.MakeRaw(fd)
	defer func() {
		_ = xterm.Restore(fd, oldState)
	}()
	if err != nil {
		return err
	}

	win.Width, win.Height, err = xterm.GetSize(fd)
	if err != nil {
		return err
	}

	if err := d.OpenTerminal(win); err != nil {
		return err
	}

	return d.session.Wait()
}

// OpenTerminal starts a login shell on the remote host.
func (d *SSHShell) OpenTerminal(win hosts.ShellWindowSize) error {
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

type singleWriter struct {
	b  io.Writer
	mu sync.Mutex
}

func (w *singleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}
