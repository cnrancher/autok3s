package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	websocketutils "github.com/cnrancher/autok3s/pkg/server/store/websocket/utils"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type Terminal struct {
	conn         *websocket.Conn
	session      *ssh.Session
	sshStdinPipe io.WriteCloser
	reader       *websocketutils.TerminalReader
}

func NewTerminal(conn *websocket.Conn) *Terminal {
	return &Terminal{
		conn: conn,
	}
}

func NewSSHClient(id, node string) (*hosts.Tunnel, error) {
	return getTunnel(id, node)
}

func (t *Terminal) StartTerminal(sshClient *hosts.Tunnel, rows, cols int) error {
	s, err := sshClient.Session()
	if err != nil {
		return err
	}
	t.session = s

	r := websocketutils.NewReader(t.conn)
	r.SetResizeFunction(t.ChangeWindowSize)
	t.reader = r
	t.session.Stdin = r
	w := websocketutils.NewWriter(t.conn)
	t.session.Stdout = w
	t.session.Stderr = w

	term := "xterm"
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err = t.session.RequestPty(term, rows, cols, modes); err != nil {
		return err
	}

	if err = t.session.Shell(); err != nil {
		return err
	}

	return nil
}

func (t *Terminal) Close() {
	if t.session != nil {
		t.session.Close()
	}
}

func (t *Terminal) WriteToTerminal(data []byte) {
	t.sshStdinPipe.Write(data)
}

func (t *Terminal) ChangeWindowSize(win *websocketutils.WindowSize) {
	if err := t.session.WindowChange(win.Height, win.Width); err != nil {
		logrus.Errorf("[ssh terminal] failed to change ssh window size: %v", err)
	}
}

func (t *Terminal) ReadMessage(ctx context.Context) error {
	return websocketutils.ReadMessage(ctx, t.conn, t.Close, t.session.Wait, t.reader.ClosedCh)
}

func getTunnel(id, node string) (*hosts.Tunnel, error) {
	// get node status from state
	state, err := common.DefaultDB.GetClusterByID(id)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("cluster %s is not exist", id)
	}
	allNodes := []autok3stypes.Node{}
	err = json.Unmarshal(state.MasterNodes, &allNodes)
	if err != nil {
		return nil, err
	}
	nodes := []autok3stypes.Node{}
	err = json.Unmarshal(state.WorkerNodes, &nodes)
	if err != nil {
		return nil, err
	}
	allNodes = append(allNodes, nodes...)
	for _, n := range allNodes {
		if n.InstanceID == node {
			dialer, err := hosts.SSHDialer(&hosts.Host{Node: n})
			if err != nil {
				return nil, err
			}
			return dialer.OpenTunnel(true)
		}
	}
	return nil, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("node %s is not found for cluster [%s]", node, id))
}
