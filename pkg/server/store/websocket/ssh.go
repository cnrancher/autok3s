package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:    10240,
	WriteBufferSize:   10240,
	HandshakeTimeout:  60 * time.Second,
	EnableCompression: true,
}

func Handler(apiOp *types.APIRequest) (types.APIObjectList, error) {
	err := handler(apiOp)
	if err != nil {
		logrus.Errorf("Error during ssh %v", err)
	}
	return types.APIObjectList{}, validation.ErrComplete
}

func handler(apiOp *types.APIRequest) error {
	queryParams := apiOp.Request.URL.Query()
	provider := queryParams.Get("provider")
	id := queryParams.Get("cluster")
	node := queryParams.Get("node")
	height := queryParams.Get("height")
	width := queryParams.Get("width")
	rows := 150
	columns := 300
	var err error
	if height != "" {
		rows, err = strconv.Atoi(height)
		if err != nil {
			return apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("invalid height %s", height))
		}
	}
	if width != "" {
		columns, err = strconv.Atoi(width)
		if err != nil {
			return apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("invalid width %s", width))
		}
	}
	if provider == "" || id == "" || node == "" {
		return apierror.NewAPIError(validation.InvalidOption, "provider, cluster, node can't be empty")
	}
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	c, err := upgrader.Upgrade(apiOp.Response, apiOp.Request, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	name := strings.Split(id, ".")[0]
	tunnel, err := getTunnel(provider, name, node)
	if err != nil {
		return err
	}
	defer tunnel.Close()
	s, err := tunnel.Session()
	if err != nil {
		return err
	}
	defer s.Close()
	s.Stdin = NewReader(c)
	w := NewWriter(c)
	s.Stdout = w
	s.Stderr = w

	term := "xterm"
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.VSTATUS:       1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := s.RequestPty(term, rows, columns, modes); err != nil {
		return err
	}

	if err := s.Shell(); err != nil {
		return err
	}

	t := time.NewTicker(30 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			_, err := w.Write([]byte("ping"))
			if err != nil {
				return nil
			}
		}
	}
}

func getTunnel(provider, name, node string) (*hosts.Tunnel, error) {
	// get node status from state
	state, err := common.DefaultDB.GetCluster(name, provider)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("[%s] cluster %s is not exist", provider, name)
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
	return nil, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster named %s of provider %s is not found", name, provider))
}
