package harvester

type Options struct {
	KubeConfigContent string `json:"kubeconfig-content,omitempty" yaml:"kubeconfig-content,omitempty"`
	KubeConfigFile    string `json:"kubeconfig-file,omitempty" yaml:"kubeconfig-file,omitempty"`
	VMNamespace       string `json:"vm-namespace,omitempty" yaml:"vm-namespace,omitempty"`
	CPUCount          int    `json:"cpu-count,omitempty" yaml:"cpu-count,omitempty"`
	MemorySize        string `json:"memory-size,omitempty" yaml:"memory-size,omitempty"`
	DiskSize          string `json:"disk-size,omitempty" yaml:"disk-size,omitempty"`
	DiskBus           string `json:"disk-bus,omitempty" yaml:"disk-bus,omitempty"`
	ImageName         string `json:"image-name,omitempty" yaml:"image-name,omitempty"`
	KeypairName       string `json:"keypair-name,omitempty" yaml:"keypair-name,omitempty"`
	NetworkType       string `json:"network-type,omitempty" yaml:"network-type,omitempty"`
	NetworkName       string `json:"network-name,omitempty" yaml:"network-name,omitempty"`
	NetworkModel      string `json:"network-model,omitempty" yaml:"network-model,omitempty"`
	InterfaceType     string `json:"interface-type,omitempty" yaml:"interface-type,omitempty"`
	CloudConfig       string `json:"cloud-config,omitempty" yaml:"cloud-config,omitempty"`
	UserData          string `json:"user-data,omitempty" yaml:"user-data,omitempty"`
	NetworkData       string `json:"network-data,omitempty" yaml:"network-data,omitempty"`
	SSHPublicKey      string `json:"ssh-public-key,omitempty" yaml:"ssh-public-key,omitempty"`
}
