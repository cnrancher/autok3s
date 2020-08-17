package hosts

import (
	"errors"
	"fmt"
	"time"

	"github.com/Jason-ZW/autok3s/pkg/types"
	"github.com/Jason-ZW/autok3s/pkg/utils"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/transport"
)

const (
	tcpNetProtocol = "tcp"
	networkKind    = "network"
)

var (
	backoff = wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   2,
		Steps:    5,
	}
)

type Host struct {
	types.Node `json:",inline"`
}

type dialer struct {
	signer     ssh.Signer
	sshKey     string
	sshAddress string
	username   string
	netConn    string
}

type DialersOptions struct {
	K8sWrapTransport transport.WrapperFunc
}

func SSHDialer(h *Host) (*dialer, error) {
	return newDialer(h, networkKind)
}

func (d *dialer) OpenTunnel() (*Tunnel, error) {
	wait.ErrWaitTimeout = errors.New(fmt.Sprintf("[dialer] calling openTunnel error. address=[%s]\n", d.sshAddress))

	var conn *ssh.Client
	var err error

	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		conn, err = d.getSSHTunnelConnection()
		if err != nil {
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("[dialer] failed to open ssh tunnel using address [%s]: %v\n", d.sshAddress, err)
	}

	return &Tunnel{conn: conn}, nil
}

func newDialer(h *Host, kind string) (*dialer, error) {
	var d *dialer

	if len(h.PublicIPAddress) <= 0 {
		return nil, errors.New("[dialer] no node IP is specified\n")
	}

	d = &dialer{
		sshAddress: fmt.Sprintf("%s:%s", h.PublicIPAddress[0], h.Port),
		username:   h.User,
	}

	if d.sshKey == "" {
		var err error
		d.sshKey, err = utils.SSHPrivateKeyPath(h.SSHKey)
		if err != nil {
			return nil, err
		}
	}

	switch kind {
	case networkKind:
		d.netConn = tcpNetProtocol
	}

	return d, nil
}

func (d *dialer) getSSHTunnelConnection() (*ssh.Client, error) {
	cfg, err := utils.GetSSHConfig(d.username, d.sshKey)
	if err != nil {
		return nil, err
	}
	// Establish connection with SSH server
	return ssh.Dial(tcpNetProtocol, d.sshAddress, cfg)
}
