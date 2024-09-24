package websocket

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/hpcloud/tail"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
)

// LogHandler log handler.
func LogHandler(apiOp *types.APIRequest) (types.APIObjectList, error) {
	if err := logHandler(apiOp); err != nil {
		return types.APIObjectList{}, err
	}
	return types.APIObjectList{}, validation.ErrComplete
}

// NewTailLog return new tail struct.
func NewTailLog(logFilePath string) (*tail.Tail, error) {
	t, err := tail.TailFile(logFilePath, tail.Config{
		Follow:    true,
		MustExist: true,
		Poll:      true,
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// CloseLog used to close log tail.
func CloseLog(t *tail.Tail) {
	_ = t.Stop()
	t.Cleanup()
}

// WriteLastLogs append logs to the log file.
func WriteLastLogs(t *tail.Tail, w http.ResponseWriter, f http.Flusher, logFilePath string) error {
	// the tail is about to close, we need to read last bytes of file to show final log
	offset, err := t.Tell()
	if err != nil {
		return err
	}
	logFile, err := os.Open(logFilePath)
	if err != nil {
		return err
	}
	_, err = logFile.Seek(offset, io.SeekCurrent)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(logFile)
	for scanner.Scan() {
		var bs = bytes.NewBufferString(fmt.Sprintf("data:%s\n\n", scanner.Text()))
		_, _ = w.Write(bs.Bytes())
		f.Flush()
	}
	CloseLog(t)
	_ = logFile.Close()
	return nil
}

// nolint: gocyclo
func logHandler(apiOp *types.APIRequest) error {
	w := apiOp.Response
	f, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("cannot support sse")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var (
		logFilePath string
		shouldClose bool
		err         error
		contextType string
		contextName string
	)

	cluster := apiOp.Request.URL.Query().Get("cluster")
	pkg := apiOp.Request.URL.Query().Get("package")
	if cluster != "" && pkg == "" {
		contextType = "cluster"
		contextName = cluster
		logFilePath, shouldClose, err = getClusterLogFile(cluster)
	} else if cluster == "" && pkg != "" {
		contextType = "package"
		contextName = pkg
		logFilePath, shouldClose, err = getAirgapDownloadLogFile(pkg)
	} else if cluster == "" && pkg == "" {
		err = apierror.NewAPIError(validation.MissingRequired, "missing the name of cluster or package")
	} else {
		err = apierror.NewAPIError(validation.InvalidOption, "only support passing cluster name or package name")
	}

	if err != nil {
		return err
	}

	// show all logs if cluster is running
	if shouldClose {
		// show all logs from file
		logFile, err := os.Open(logFilePath)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(logFile)
		for scanner.Scan() {
			var bs = bytes.NewBufferString(fmt.Sprintf("data:%s\n\n", scanner.Text()))
			_, _ = w.Write(bs.Bytes())
			f.Flush()
		}
		_, _ = w.Write([]byte("event: close\ndata: close\n\n"))
		return logFile.Close()
	}

	t, err := NewTailLog(logFilePath)
	if err != nil {
		return err
	}

	result := make(chan *common.LogEvent)
	go common.DefaultDB.Log(apiOp, contextType, result)

	for {
		select {
		case s, ok := <-result:
			if !ok {
				_, _ = w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
			if s.ContextName == contextName && s.ContextType == contextType {
				err = WriteLastLogs(t, w, f, logFilePath)
				if err != nil {
					_, _ = w.Write([]byte("event: close\ndata: close\n\n"))
					return err
				}
				close(result)
				_, _ = w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
		case <-apiOp.Context().Done():
			CloseLog(t)
			close(result)
			return nil
		case line, ok := <-t.Lines:
			if !ok {
				_, _ = w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
			var bs = bytes.NewBufferString(fmt.Sprintf("data:%s\n\n", line.Text))
			_, _ = w.Write(bs.Bytes())
			f.Flush()
		}
	}
}

func getClusterLogFile(cluster string) (string, bool, error) {
	logFilePath := common.GetClusterLogFilePath(cluster)
	shouldClose := false
	state, err := common.DefaultDB.GetClusterByID(cluster)
	if err != nil {
		return "", shouldClose, apierror.WrapAPIError(err, validation.ServerError, "failed to get cluster by ID")
	}
	if state == nil {
		return "", shouldClose, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not exist", cluster))
	}
	if state.Status != common.StatusCreating && state.Status != common.StatusUpgrading {
		shouldClose = true
	}
	return logFilePath, shouldClose, nil
}

func getAirgapDownloadLogFile(name string) (string, bool, error) {
	logFilePath := airgap.GetDownloadFilePath(name)
	shouldClose := false
	pkgs, err := common.DefaultDB.ListPackages(&name)
	if err != nil {
		return "", shouldClose, apierror.WrapAPIError(err, validation.ServerError, "failed to get package by ID")
	}
	if pkgs[0].State == common.PackageActive {
		shouldClose = true
	}
	return logFilePath, shouldClose, nil
}
