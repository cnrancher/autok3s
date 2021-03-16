// +build darwin linux

package kubectl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	websocketutil "github.com/cnrancher/autok3s/pkg/server/store/websocket/utils"

	"github.com/creack/pty"
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
	defer c.Close()

	s := &Shell{
		conn: c,
	}
	return s.startTerminal(apiOp.Request.Context(), rows, columns)
}

func (s *Shell) startTerminal(ctx context.Context, rows, cols int) error {
	os.Symlink(fmt.Sprintf("%s kubectl", os.Args[0]), fmt.Sprintf("%s/kubectl", common.CfgPath))

	kubeBash := exec.CommandContext(ctx, "sh")
	// Start the command with a pty.
	p, err := pty.StartWithSize(kubeBash, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return err
	}
	s.ptmx = p
	w := websocketutil.NewWriter(s.conn)
	go io.Copy(w, s.ptmx)
	return websocketutil.ReadMessage(ctx, s.conn, s.Close, s.WriteToShell, s.ChangeSize)
}

func (s *Shell) Close() {
	if s.ptmx != nil {
		s.ptmx.Close()
	}
}

func (s *Shell) ChangeSize(win *websocketutil.WindowSize) {
	pty.Setsize(s.ptmx, &pty.Winsize{
		Rows: uint16(win.Height),
		Cols: uint16(win.Width),
	})
}

func (s *Shell) WriteToShell(data []byte) {
	s.ptmx.Write(data)
}
