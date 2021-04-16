package ssh

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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

	tunnel, err := NewSSHClient(id, node)
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Close()
	}()

	terminal := NewTerminal(c)
	err = terminal.StartTerminal(tunnel, rows, columns)
	if err != nil {
		return err
	}

	return terminal.ReadMessage(apiOp.Context())
}
