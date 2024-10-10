package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	KubeExplorerCommand = "kube-explorer"
)

// EnableExplorer will start kube-explorer with random port for specified K3s cluster
func EnableExplorer(ctx context.Context, config string) error {
	if _, ok := ExplorerWatchers[config]; ok {
		return fmt.Errorf("kube-explorer for cluster %s has already started", config)
	}
	if err := CheckCommandExist(KubeExplorerCommand); err != nil {
		return err
	}

	// command execution validate
	if err := checkExplorerCmd(); err != nil {
		return err
	}

	// save config for kube-explorer
	exp, err := DefaultDB.GetExplorer(config)
	if err != nil {
		return err
	}

	if exp == nil || !exp.Enabled {
		exp = &Explorer{
			ContextName: config,
			Enabled:     true,
		}
		if err = DefaultDB.SaveExplorer(exp); err != nil {
			return err
		}
	}

	// start kube-explorer
	explorerCtx, cancel := context.WithCancel(ctx)
	ExplorerWatchers[config] = cancel

	if _, err := StartKubeExplorer(explorerCtx, config); err != nil {
		return err
	}

	return nil
}

// DisableExplorer will stop kube-explorer server for specified K3s cluster
func DisableExplorer(config string) error {
	cancelFunc, ok := ExplorerWatchers[config]
	if !ok {
		return fmt.Errorf("cann't disable unactive kube-explorer for cluster %s", config)
	}

	// update kube-explorer settings
	exp, err := DefaultDB.GetExplorer(config)
	if err != nil {
		return err
	}
	if exp != nil && exp.Enabled {
		// stop kube-explorer
		cancelFunc()
		delete(ExplorerWatchers, config)
	}
	if exp == nil || exp.Enabled {
		err = DefaultDB.SaveExplorer(&Explorer{
			ContextName: config,
			Enabled:     false,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// InitExplorer will start kube-explorer server for all K3s clusters which enabled explorer setting
func InitExplorer(ctx context.Context) {
	expList, err := DefaultDB.ListExplorer()
	if err != nil {
		logrus.Errorf("get kube-explorer settings error: %v", err)
		return
	}
	for _, exp := range expList {
		if exp.Enabled {
			logrus.Infof("start kube-explorer for cluster %s", exp.ContextName)
			if err = EnableExplorer(ctx, exp.ContextName); err != nil {
				logrus.Errorf("failed to start kube-explorer for cluster %s: %v", exp.ContextName, err)
			}
		}
	}
}

// StartKubeExplorer start kube-explorer server listen on specified port
func StartKubeExplorer(ctx context.Context, clusterID string) (chan int, error) {
	socketName := GetSocketName(clusterID)
	explorer := exec.CommandContext(ctx, KubeExplorerCommand, fmt.Sprintf("--kubeconfig=%s", filepath.Join(CfgPath, KubeCfgFile)),
		fmt.Sprintf("--context=%s", clusterID), fmt.Sprintf("--bind-address=%s", socketName))
	explorer.Stdout = os.Stdout
	explorer.Stderr = os.Stderr
	explorer.Cancel = func() error {
		return explorer.Process.Signal(os.Interrupt)
	}
	explorer.WaitDelay = 10 * time.Second
	if err := explorer.Start(); err != nil {
		logrus.Errorf("fail to start kube-explorer for cluster %s: %v", clusterID, err)
	}
	logrus.Infof("kube-explorer for %s K3s cluster will listen on %s ...", clusterID, socketName)
	stopChan := make(chan int)
	go func() {
		_ = explorer.Wait()
		close(stopChan)
	}()
	return stopChan, nil
}

func CheckCommandExist(cmd string) error {
	_, err := exec.LookPath(cmd)
	return err
}

func checkExplorerCmd() error {
	explorerVersion := exec.Command(KubeExplorerCommand, "--version")
	return explorerVersion.Run()
}
