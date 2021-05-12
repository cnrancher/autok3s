package ssh

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
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
		logrus.Errorf("error during ssh %v", err)
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
	defer func() {
		_ = c.Close()
	}()

	dialer, err := newDialer(id, node, c)
	if err != nil {
		return err
	}
	defer dialer.Close()

	dialer.SetDefaultSize(rows, columns)

	err = dialer.Terminal()
	if err != nil {
		return err
	}

	return dialer.ReadMessage(apiOp.Context())
}

func newDialer(id, node string, conn *websocket.Conn) (*hosts.WebSocketDialer, error) {
	// get exist cluster's state from database.
	state, err := common.DefaultDB.GetClusterByID(id)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("cluster %s is not exist", id)
	}

	// aggregate exist cluster's nodes.
	allNodes := make([]autok3stypes.Node, 0)
	err = json.Unmarshal(state.MasterNodes, &allNodes)
	if err != nil {
		return nil, err
	}
	nodes := make([]autok3stypes.Node, 0)
	err = json.Unmarshal(state.WorkerNodes, &nodes)
	if err != nil {
		return nil, err
	}
	allNodes = append(allNodes, nodes...)

	var wsDialer *hosts.WebSocketDialer

	// find the matching node and open dialer.
	for _, n := range allNodes {
		if n.InstanceID == node {
			if state.Provider == "k3d" {
				dialer, err := hosts.NewDockerDialer(&n)
				if err != nil {
					return nil, err
				}
				wsDialer = hosts.NewWebSocketDialer(conn, dialer)
				return wsDialer, nil
			}
			dialer, err := hosts.NewSSHDialer(&n, true)
			if err != nil {
				return nil, err
			}
			wsDialer = hosts.NewWebSocketDialer(conn, dialer)
			return wsDialer, nil
		}
	}
	return nil, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("node %s is not found for cluster [%s]", node, id))
}
