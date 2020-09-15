package common

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	BindPrefix         = "autok3s.providers.%s.%s"
	ConfigFile         = "config.yaml"
	StateFile          = ".state"
	KubeCfgFile        = ".kube/config"
	KubeCfgTempName    = "autok3s-temp"
	K3sManifestsDir    = "/var/lib/rancher/k3s/server/manifests"
	MasterInstanceName = "autok3s.%s.master"
	WorkerInstanceName = "autok3s.%s.worker"
)

var (
	Debug   = false
	CfgPath = "/var/lib/rancher/autok3s"
	Backoff = wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   1,
		Steps:    5,
	} // retry 5 times, total 120 seconds.
)
