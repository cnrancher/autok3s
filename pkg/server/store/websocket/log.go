package websocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/hpcloud/tail"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
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
	_, err = logFile.Seek(offset, os.SEEK_CUR)
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
	cluster := apiOp.Request.URL.Query().Get("cluster")
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

	logFilePath := common.GetLogFilePath(cluster)
	state, err := common.DefaultDB.GetClusterByID(cluster)
	if err != nil {
		return err
	}
	if state == nil {
		return apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not exist", cluster))
	}

	// show all logs if cluster is running
	if state.Status != common.StatusCreating && state.Status != common.StatusUpgrading {
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
	go common.DefaultDB.Log(apiOp, result)

	for {
		select {
		case s, ok := <-result:
			if !ok {
				_, _ = w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
			if s.ContextName == cluster {
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
