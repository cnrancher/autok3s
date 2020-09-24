package common

import (
	"time"

	"github.com/cnrancher/autok3s/pkg/utils"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	BindPrefix           = "autok3s.providers.%s.%s"
	ConfigFile           = "config.yaml"
	StateFile            = ".state"
	KubeCfgFile          = ".kube/config"
	KubeCfgTempName      = "autok3s-temp"
	K3sManifestsDir      = "/var/lib/rancher/k3s/server/manifests"
	MasterInstanceName   = "autok3s.%s.master"
	WorkerInstanceName   = "autok3s.%s.worker"
	TagClusterPrefix     = "autok3s-"
	K3sTagNameInternalIP = "k3s.io/internal-ip"
	K3sRoleMasterValue   = "master"
)

var (
	Debug   = false
	CfgPath = utils.UserHome() + "/.autok3s"
	Backoff = wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   1,
		Steps:    5,
	} // retry 5 times, total 120 seconds.
)
