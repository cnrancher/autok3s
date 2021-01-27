package websocket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/creack/pty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

func KubeHandler(apiOp *types.APIRequest) (types.APIObject, error) {
	err := ptyHandler(apiOp)
	if err != nil {
		logrus.Errorf("Error during kubectl handler %v", err)
	}
	return types.APIObject{}, validation.ErrComplete
}

func ptyHandler(apiOp *types.APIRequest) error {
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	c, err := upgrader.Upgrade(apiOp.Response, apiOp.Request, nil)
	if err != nil {
		return err
	}
	defer c.Close()

	w := NewWriter(c)
	reader := NewReader(c)

	ctx, cancel := context.WithCancel(apiOp.Request.Context())
	go func() {
		newPty(ctx, w, reader, cancel)
	}()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			_, err := w.Write([]byte("ping"))
			if err != nil {
				return err
			}
		}
	}
}

func newPty(ctx context.Context, w io.Writer, reader io.Reader, cancel context.CancelFunc) error {
	// symbolic link for kubectl
	os.Symlink(fmt.Sprintf("%s kubectl", os.Args[0]), fmt.Sprintf("%s/kubectl", common.CfgPath))

	kubeBash := exec.CommandContext(ctx, "sh")
	// Start the command with a pty.
	ptmx, err := pty.StartWithSize(kubeBash, &pty.Winsize{
		Cols: 300,
		Rows: 150,
	})
	if err != nil {
		return err
	}

	defer ptmx.Close()

	go func() {
		io.Copy(ptmx, reader)
		cancel()
	}()
	io.Copy(w, ptmx)
	return nil
}
