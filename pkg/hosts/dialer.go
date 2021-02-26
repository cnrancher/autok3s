package hosts

import (
	"errors"
	"fmt"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/transport"
)

const (
	tcpNetProtocol = "tcp"
	networkKind    = "network"
)

type Host struct {
	types.Node `json:",inline"`
}

type Dialer struct {
	sshKey     string
	sshCert    string
	sshAddress string
	username   string
	password   string
	passphrase string
	netConn    string

	useSSHAgentAuth bool
}

type DialersOptions struct {
	K8sWrapTransport transport.WrapperFunc
}

func SSHDialer(h *Host) (*Dialer, error) {
	return newDialer(h, networkKind)
}

func (d *Dialer) OpenTunnel(timeout bool) (*Tunnel, error) {
	wait.ErrWaitTimeout = fmt.Errorf("[dialer] calling openTunnel error. address [%s]", d.sshAddress)

	var conn *ssh.Client
	var err error

	if err := wait.ExponentialBackoff(common.Backoff, func() (bool, error) {
		conn, err = d.getSSHTunnelConnection(timeout)
		if err != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("[dialer] failed to open ssh tunnel using address [%s]: %v", d.sshAddress, err)
	}

	return &Tunnel{conn: conn}, nil
}

func newDialer(h *Host, kind string) (*Dialer, error) {
	var d *Dialer

	if len(h.PublicIPAddress) <= 0 {
		return nil, errors.New("[dialer] no node IP is specified")
	}

	d = &Dialer{
		sshAddress:      fmt.Sprintf("%s:%s", h.PublicIPAddress[0], h.SSHPort),
		username:        h.SSHUser,
		password:        h.SSHPassword,
		passphrase:      h.SSHKeyPassphrase,
		useSSHAgentAuth: h.SSHAgentAuth,
		sshCert:         h.SSHCert,
	}

	if d.password == "" && d.sshKey == "" && !d.useSSHAgentAuth && len(h.SSHKeyPath) > 0 {
		var err error
		d.sshKey, err = utils.SSHPrivateKeyPath(h.SSHKeyPath)
		if err != nil {
			return nil, err
		}

		if d.sshCert == "" && len(h.SSHCertPath) > 0 {
			d.sshCert, err = utils.SSHCertificatePath(h.SSHCertPath)
			if err != nil {
				return nil, err
			}
		}
	}

	switch kind {
	case networkKind:
		d.netConn = tcpNetProtocol
	}

	return d, nil
}

func (d *Dialer) getSSHTunnelConnection(t bool) (*ssh.Client, error) {
	timeout := time.Duration((common.Backoff.Steps - 1) * int(common.Backoff.Duration))
	if !t {
		timeout = 0
	}

	cfg, err := utils.GetSSHConfig(d.username, d.sshKey, d.passphrase, d.sshCert, d.password, timeout, d.useSSHAgentAuth)
	if err != nil {
		return nil, err
	}
	// establish connection with SSH server.
	return ssh.Dial(tcpNetProtocol, d.sshAddress, cfg)
}
