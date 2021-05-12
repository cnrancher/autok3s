// +build darwin linux

package kubectl

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/cnrancher/autok3s/pkg/hosts"

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

type Shell struct {
	conn *websocket.Conn
	ptmx *os.File
}

func KubeHandler(apiOp *types.APIRequest) (types.APIObject, error) {
	err := ptyHandler(apiOp)
	if err != nil {
		logrus.Errorf("error during kubectl handler %v", err)
	}
	return types.APIObject{}, validation.ErrComplete
}

func ptyHandler(apiOp *types.APIRequest) error {
	queryParams := apiOp.Request.URL.Query()
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

	dialer, err := hosts.NewPtyDialer(exec.CommandContext(apiOp.Request.Context(), "bash"))
	if err != nil {
		return err
	}
	wsDialer := hosts.NewWebSocketDialer(c, dialer)
	wsDialer.SetDefaultSize(rows, columns)

	err = wsDialer.Terminal()
	if err != nil {
		return err
	}

	aliasCmd := fmt.Sprintf("alias kubectl='kubectl --context %s'\n", apiOp.Name)
	aliasCmd = fmt.Sprintf("%salias k='kubectl --context %s'\n", aliasCmd, apiOp.Name)

	err = wsDialer.Write([]byte(aliasCmd))
	if err != nil {
		return err
	}

	return wsDialer.ReadMessage(apiOp.Context())
}
