package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	k3dutil "github.com/k3d-io/k3d/v5/cmd/util"
	"github.com/sirupsen/logrus"
)

const (
	KubeExplorerCommand = "kube-explorer"
)

// EnableExplorer will start kube-explorer with random port for specified K3s cluster
func EnableExplorer(ctx context.Context, config string) (int, error) {
	if _, ok := ExplorerWatchers[config]; ok {
		return 0, fmt.Errorf("kube-explorer for cluster %s has already started", config)
	}
	if err := CheckCommandExist(KubeExplorerCommand); err != nil {
		return 0, err
	}

	// command execution validate
	if err := checkExplorerCmd(); err != nil {
		return 0, err
	}

	// save config for kube-explorer
	exp, err := DefaultDB.GetExplorer(config)
	if err != nil {
		return 0, err
	}

	if exp == nil || !exp.Enabled {
		var port int
		if exp == nil {
			port, err = k3dutil.GetFreePort()
			if err != nil {
				return 0, err
			}
		} else {
			port = exp.Port
		}
		exp = &Explorer{
			ContextName: config,
			Port:        port,
			Enabled:     true,
		}
		if err = DefaultDB.SaveExplorer(exp); err != nil {
			return 0, err
		}
	}

	// start kube-explorer
	explorerCtx, cancel := context.WithCancel(ctx)
	ExplorerWatchers[config] = cancel
	go func(ctx context.Context, config string, port int) {
		_ = StartKubeExplorer(ctx, config, port)
	}(explorerCtx, config, exp.Port)
	return exp.Port, nil
}

// DisableExplorer will stop kube-explorer server for specified K3s cluster
func DisableExplorer(config string) error {
	if _, ok := ExplorerWatchers[config]; !ok {
		return fmt.Errorf("cann't disable unactive kube-explorer for cluster %s", config)
	}
	// update kube-explorer settings
	exp, err := DefaultDB.GetExplorer(config)
	if err != nil {
		return err
	}
	if exp == nil || exp.Enabled {
		var port int
		if exp == nil {
			port, err = k3dutil.GetFreePort()
			if err != nil {
				return err
			}
		} else {
			port = exp.Port
		}
		err = DefaultDB.SaveExplorer(&Explorer{
			ContextName: config,
			Port:        port,
			Enabled:     false,
		})
		if err != nil {
			return err
		}
	}

	// stop kube-explorer
	ExplorerWatchers[config]()
	delete(ExplorerWatchers, config)
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
			go func(ctx context.Context, name string) {
				if _, err = EnableExplorer(ctx, name); err != nil {
					logrus.Errorf("failed to start kube-explorer for cluster %s: %v", name, err)
				}
			}(ctx, exp.ContextName)
		}
	}
}

// StartKubeExplorer start kube-explorer server listen on specified port
func StartKubeExplorer(ctx context.Context, config string, port int) error {
	explorer := exec.CommandContext(ctx, KubeExplorerCommand, fmt.Sprintf("--kubeconfig=%s", filepath.Join(CfgPath, KubeCfgFile)),
		fmt.Sprintf("--context=%s", config), fmt.Sprintf("--http-listen-port=%d", port), "--https-listen-port=0")
	explorer.Stdout = os.Stdout
	explorer.Stderr = os.Stderr
	if err := explorer.Start(); err != nil {
		logrus.Errorf("fail to start kube-explorer for cluster %s: %v", config, err)
	}
	logrus.Infof("kube-explorer for %s K3s cluster will listen on 127.0.0.1:%d ...", config, port)
	return explorer.Wait()
}

func CheckCommandExist(cmd string) error {
	_, err := exec.LookPath(cmd)
	return err
}

func checkExplorerCmd() error {
	explorerVersion := exec.Command(KubeExplorerCommand, "--version")
	return explorerVersion.Run()
}
