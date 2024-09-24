package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cnrancher/autok3s/pkg/settings"
	k3dutil "github.com/k3d-io/k3d/v5/cmd/util"
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/clientcmd"
)

const HelmDashboardCommand = "helm-dashboard"

var dashboardBindAddress = "127.0.0.1"

func SwitchDashboard(ctx context.Context, enabled string) error {
	if enabled == "true" {
		// check if cluster list is empty
		clusters, err := DefaultDB.ListCluster("")
		if err != nil {
			return err
		}
		if len(clusters) == 0 {
			return errors.New("cannot enable helm-dashboard with empty cluster list")
		}
	}
	if err := CheckCommandExist(HelmDashboardCommand); err != nil {
		return err
	}

	// command execution validate
	if err := checkDashboardCmd(); err != nil {
		return err
	}

	isEnabled := settings.HelmDashboardEnabled.Get()
	if !strings.EqualFold(isEnabled, enabled) {
		if err := settings.HelmDashboardEnabled.Set(enabled); err != nil {
			return err
		}
		if enabled == "true" {
			enableDashboard(ctx)
		} else {
			if err := settings.HelmDashboardEnabled.Set("false"); err != nil {
				return err
			}
			logrus.Info("Shutting down helm-dashboard...")
			DashboardCanceled()
		}
	}

	return nil
}

func InitDashboard(ctx context.Context) {
	isEnabled := settings.HelmDashboardEnabled.Get()
	if isEnabled == "true" {
		enableDashboard(ctx)
	}
}

func StartHelmDashboard(ctx context.Context, port string) error {
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, filepath.Join(CfgPath, KubeCfgFile))
	if os.Getenv("AUTOK3S_HELM_DASHBOARD_ADDRESS") != "" {
		dashboardBindAddress = os.Getenv("AUTOK3S_HELM_DASHBOARD_ADDRESS")
	}
	dashboard := exec.CommandContext(ctx, HelmDashboardCommand, fmt.Sprintf("--bind=%s", dashboardBindAddress), "-b", fmt.Sprintf("--port=%s", port))
	dashboard.Stdout = os.Stdout
	dashboard.Stderr = os.Stderr
	if err := dashboard.Start(); err != nil {
		logrus.Errorf("fail to start helm-dashboard: %v", err)
	}
	logrus.Infof("helm-dashboard will listen on %s:%s ...", dashboardBindAddress, port)
	return dashboard.Wait()
}

func checkDashboardCmd() error {
	explorerVersion := exec.Command(HelmDashboardCommand, "--version")
	return explorerVersion.Run()
}

func enableDashboard(ctx context.Context) {
	clusters, err := DefaultDB.ListCluster("")
	if err != nil {
		logrus.Errorf("failed to list clusters: %v", err)
		return
	}
	// disable helm-dashboard if there's no cluster context
	if len(clusters) == 0 {
		if err := settings.HelmDashboardEnabled.Set("false"); err != nil {
			logrus.Errorf("failed to disable helm-dashboard due to empty cluster list: %v", err)
			return
		}
		logrus.Warn("disabled helm-dashboard with empty cluster list")
		return
	}
	dashboardPort := settings.HelmDashboardPort.Get()
	if dashboardPort == "" {
		freePort, err := k3dutil.GetFreePort()
		if err != nil {
			logrus.Errorf("failed to get free port for helm-dashboard: %v", err)
			return
		}
		dashboardPort = strconv.Itoa(freePort)
		err = settings.HelmDashboardPort.Set(dashboardPort)
		if err != nil {
			logrus.Errorf("failed to save helm-dashboard port to settings: %v", err)
			return
		}
	}
	logrus.Info("Enable helm-dashboard server...")
	dashboardCtx, cancel := context.WithCancel(ctx)
	DashboardCanceled = cancel
	go func(ctx context.Context, port string) {
		_ = StartHelmDashboard(ctx, port)
	}(dashboardCtx, dashboardPort)
}
