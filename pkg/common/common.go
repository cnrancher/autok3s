package common

const (
	MasterInstancePrefix = "autok3s.%s.m"                   // autok3s.<cluster>.m
	WorkerInstancePrefix = "autok3s.%s.w"                   // autok3s.<cluster>.w
	MasterInstanceName   = MasterInstancePrefix + "[%d,%d]" // autok3s.<cluster>.m<index>
	WorkerInstanceName   = WorkerInstancePrefix + "[%d,%d]" // autok3s.<cluster>.w<index>
	WildcardInstanceName = "autok3s.%s.*"                   // autok3s.<cluster>.*
	BindPrefix           = "autok3s.providers.%s.%s"
	ConfigFile           = "config.yaml"
	StateFile            = ".state"
	KubeCfgFile          = ".kube/config"
)

var (
	CfgPath = "/var/lib/rancher/autok3s"
)
