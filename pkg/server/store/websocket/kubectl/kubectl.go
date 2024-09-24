//go:build darwin || linux
// +build darwin linux

package kubectl

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/hosts/dialer"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:    10240,
	WriteBufferSize:   10240,
	HandshakeTimeout:  60 * time.Second,
	EnableCompression: true,
}

// Shell struct for shell.
type Shell struct {
	// conn *websocket.Conn
	// ptmx *os.File
}

// KubeHandler kubectl handler for websocket.
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

	upgrader.CheckOrigin = func(_ *http.Request) bool {
		return true
	}
	c, err := upgrader.Upgrade(apiOp.Response, apiOp.Request, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = c.Close()
	}()

	dialer, err := dialer.NewPtyShell(exec.CommandContext(apiOp.Request.Context(), "bash"))
	if err != nil {
		return err
	}
	wsDialer := hosts.NewWebSocketDialer(c, dialer)

	err = wsDialer.Terminal(rows, columns)
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
