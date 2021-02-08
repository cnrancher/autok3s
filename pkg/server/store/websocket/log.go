package websocket

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/fsnotify/fsnotify"
	"github.com/hpcloud/tail"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

func LogHandler(apiOp *types.APIRequest) (types.APIObjectList, error) {
	if err := logHandler(apiOp); err != nil {
		return types.APIObjectList{}, err
	}
	return types.APIObjectList{}, validation.ErrComplete
}

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

	// check cluster is running/failed state
	stateDir := common.GetClusterStatePath()
	logFilePath := filepath.Join(common.GetLogPath(), cluster)
	if !hasProcessStateFile(cluster) {
		// show all logs from file
		logFile, err := os.Open(logFilePath)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(logFile)
		for scanner.Scan() {
			var bs = bytes.NewBufferString(fmt.Sprintf("data:%s\n\n", scanner.Text()))
			w.Write(bs.Bytes())
			f.Flush()
		}
		w.Write([]byte("event: close\ndata: close\n\n"))
		return logFile.Close()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() {
		logrus.Debugf("close watch")
		watcher.Close()
	}()
	err = watcher.Add(stateDir)
	if err != nil {
		return err
	}

	t, err := tail.TailFile(logFilePath, tail.Config{
		Follow:    true,
		MustExist: true,
		Poll:      true,
	})
	if err != nil {
		return err
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
			if event.Op == fsnotify.Remove && isProcessState(event.Name, cluster) {
				logrus.Infof("ready to close cluster %s logs", cluster)
				// the tail is about to close, we need to read last bytes of file to show final log
				offset, err := t.Tell()
				if err != nil {
					w.Write([]byte("event: close\ndata: close\n\n"))
					return err
				}
				logFile, err := os.Open(logFilePath)
				if err != nil {
					w.Write([]byte("event: close\ndata: close\n\n"))
					return err
				}
				_, err = logFile.Seek(offset, os.SEEK_CUR)
				if err != nil {
					w.Write([]byte("event: close\ndata: close\n\n"))
					return err
				}
				scanner := bufio.NewScanner(logFile)
				for scanner.Scan() {
					var bs = bytes.NewBufferString(fmt.Sprintf("data:%s\n\n", scanner.Text()))
					w.Write(bs.Bytes())
					f.Flush()
				}
				t.Stop()
				t.Cleanup()
				logFile.Close()
				logrus.Infof("close log data")
				w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
		case <-apiOp.Context().Done():
			logrus.Debug("request close from client")
			t.Stop()
			t.Cleanup()
			return nil
		case line, ok := <-t.Lines:
			if !ok {
				w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
			var bs = bytes.NewBufferString(fmt.Sprintf("data:%s\n\n", line.Text))
			w.Write(bs.Bytes())
			f.Flush()
		}
	}
}

func isProcessState(name, cluster string) bool {
	return name == filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", cluster, common.StatusCreating)) ||
		name == filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", cluster, common.StatusJoin))
}

func hasProcessStateFile(cluster string) bool {
	_, err := os.Stat(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", cluster, common.StatusCreating)))
	if err == nil {
		return true
	}
	_, err = os.Stat(filepath.Join(common.GetClusterStatePath(), fmt.Sprintf("%s_%s", cluster, common.StatusJoin)))
	if err == nil {
		return true
	}
	return false
}
