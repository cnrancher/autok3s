package common

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	BindPrefix      = "autok3s.providers.%s.%s"
	ConfigFile      = "config.yaml"
	StateFile       = ".state"
	KubeCfgFile     = ".kube/config"
	KubeCfgTempName = "autok3s-temp"
	K3sManifestsDir = "/var/lib/rancher/k3s/server/manifests"
)

var (
	CfgPath = "/Users/ouomouya/Documents/autoK3s-config/autok3s"
	// retry 5 times, total 150 seconds.
	Backoff = wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   1,
		Steps:    5,
	}
)
