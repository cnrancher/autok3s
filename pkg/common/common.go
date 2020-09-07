package common

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	MasterInstancePrefix = "autok3s.%s.m" // autok3s.<cluster.region>.m
	WorkerInstancePrefix = "autok3s.%s.w" // autok3s.<cluster.region>.w
	WildcardInstanceName = "autok3s.%s.*" // autok3s.<cluster.region>.*
	BindPrefix           = "autok3s.providers.%s.%s"
	ConfigFile           = "config.yaml"
	StateFile            = ".state"
	KubeCfgFile          = ".kube/config"
	KubeCfgTempName      = "autok3s-temp"
	K3sManifestsDir      = "/var/lib/rancher/k3s/server/manifests"
)

var (
	CfgPath = "/var/lib/rancher/autok3s"
	// retry 5 times, total 120 seconds.
	Backoff = wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   1,
		Steps:    5,
	}
)
