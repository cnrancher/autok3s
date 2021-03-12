package websocket

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

func nativeLogHandler(context context.Context, w http.ResponseWriter, f http.Flusher, cluster, provider string) error {
	stateDir := filepath.Join(common.GetLogPath(), provider)
	logFilePath := filepath.Join(common.GetLogPath(), cluster)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() {
		watcher.Close()
	}()
	err = watcher.Add(stateDir)
	if err != nil {
		return err
	}

	t, err := NewTailLog(logFilePath)
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
				err = WriteLastLogs(t, w, f, logFilePath)
				if err != nil {
					w.Write([]byte("event: close\ndata: close\n\n"))
					return err
				}
				w.Write([]byte("event: close\ndata: close\n\n"))
				return nil
			}
		case <-context.Done():
			logrus.Info("request close from client")
			CloseLog(t)
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
	return name == filepath.Join(getStatePath(), fmt.Sprintf("%s_%s", cluster, common.StatusCreating)) ||
		name == filepath.Join(getStatePath(), fmt.Sprintf("%s_%s", cluster, common.StatusUpgrading))
}

func getStatePath() string {
	return filepath.Join(common.GetLogPath(), "native")
}
