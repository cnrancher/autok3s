package common

import (
	"context"
	"path/filepath"
	"time"

	"github.com/cnrancher/autok3s/pkg/utils"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// KubeCfgFile default kube config file path.
	KubeCfgFile = ".kube/config"
	// KubeCfgTempName default temp kube config file name prefix.
	KubeCfgTempName = "autok3s-temp-*"
	// K3sManifestsDir k3s manifests dir.
	K3sManifestsDir = "/var/lib/rancher/k3s/server/manifests"
	// MasterInstanceName master instance name.
	MasterInstanceName = "autok3s.%s.master"
	// WorkerInstanceName worker instance name.
	WorkerInstanceName = "autok3s.%s.worker"
	// TagClusterPrefix cluster's tag prefix.
	TagClusterPrefix = "autok3s-"
	// StatusRunning instance running status.
	StatusRunning = "Running"
	// StatusCreating instance creating status.
	StatusCreating = "Creating"
	// StatusMissing instance missing status.
	StatusMissing = "Missing"
	// StatusFailed instance failed status.
	StatusFailed = "Failed"
	// StatusUpgrading instance upgrading status.
	StatusUpgrading = "Upgrading"
	// StatusRemoving instance removing status.
	StatusRemoving = "Removing"
	// UsageInfoTitle usage info title.
	UsageInfoTitle = "=========================== Prompt Info ==========================="
	// UsageContext usage info context.
	UsageContext = "Use 'autok3s kubectl config use-context %s'"
	// UsagePods usage  info pods.
	UsagePods = "Use 'autok3s kubectl get pods -A' get POD status`"
	// DBFolder default database dir.
	DBFolder = ".db"
	// DBFile default database file.
	DBFile = "autok3s.db"
)

var (
	// Debug used to enable log debug level.
	Debug = false
	// CfgPath default config file dir.
	CfgPath = filepath.Join(utils.UserHome(), ".autok3s")
	// Backoff default backoff variable, retry 20 times, total 570 seconds.
	Backoff = wait.Backoff{
		Duration: 30 * time.Second,
		Factor:   1,
		Steps:    20,
	}
	// DefaultDB default database store.
	DefaultDB        *Store
	ExplorerWatchers map[string]context.CancelFunc

	FileManager *ConfigFileManager
)

// GetDefaultSSHKeyPath returns default ssh key path.
func GetDefaultSSHKeyPath(clusterName, providerName string) string {
	return filepath.Join(CfgPath, providerName, "clusters", clusterName, "id_rsa")
}

// GetDefaultSSHPublicKeyPath returns default public key path.
func GetDefaultSSHPublicKeyPath(clusterName, providerName string) string {
	return filepath.Join(CfgPath, providerName, "clusters", clusterName, "id_rsa.pub")
}

// GetClusterPath returns default cluster path.
func GetClusterPath(clusterName, providerName string) string {
	return filepath.Join(CfgPath, providerName, "clusters", clusterName)
}

// GetDataSource return default database file path.
func GetDataSource() string {
	return filepath.Join(CfgPath, DBFolder, DBFile)
}
