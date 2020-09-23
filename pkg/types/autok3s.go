package types

type AutoK3s struct {
	Clusters []Cluster `json:"clusters" yaml:"clusters"`
}

type Cluster struct {
	Metadata `json:",inline" mapstructure:",squash"`
	Options  interface{} `json:"options,omitempty"`

	Status `json:"status" yaml:"status"`
}

type Metadata struct {
	Name                   string `json:"name" yaml:"name"`
	Provider               string `json:"provider" yaml:"provider"`
	Master                 string `json:"master" yaml:"master"`
	Worker                 string `json:"worker" yaml:"worker"`
	Token                  string `json:"token,omitempty" yaml:"token,omitempty"`
	UI                     string `json:"ui,omitempty" yaml:"ui,omitempty"`
	IP                     string `json:"ip,omitempty" yaml:"ip,omitempty"`
	Repo                   string `json:"repo,omitempty" yaml:"repo,omitempty"`
	ClusterCIDR            string `json:"cluster-cidr,omitempty" yaml:"cluster-cidr,omitempty"`
	CloudControllerManager string `json:"cloud-controller-manager,omitempty" yaml:"cloud-controller-manager,omitempty"`
	MasterExtraArgs        string `json:"master-extra-args,omitempty" yaml:"master-extra-args,omitempty"`
	WorkerExtraArgs        string `json:"worker-extra-args,omitempty" yaml:"worker-extra-args,omitempty"`
	Registries             string `json:"registries,omitempty" yaml:"registries,omitempty"`
	DataStore              string `json:"datastore,omitempty" yaml:"datastore,omitempty"`
	K3sVersion             string `json:"k3s-version,omitempty" yaml:"k3s-version,omitempty"`
	InstallScript          string `json:"installScript,omitempty" yaml:"installScript,omitempty" default:"https://get.k3s.io"`
	Mirror                 string `json:"mirror,omitempty" yaml:"mirror,omitempty"`
	DockerMirror           string `json:"dockerMirror,omitempty" yaml:"dockerMirror,omitempty"`
	Network                string `json:"network,omitempty" yaml:"network,omitempty"`
}

type Status struct {
	MasterNodes []Node `json:"master-nodes,omitempty"`
	WorkerNodes []Node `json:"worker-nodes,omitempty"`
}

type Node struct {
	SSH `json:",inline"`

	Master            bool     `json:"master,omitempty" yaml:"master,omitempty"`
	InstanceID        string   `json:"instance-id,omitempty" yaml:"instance-id,omitempty"`
	InstanceStatus    string   `json:"instance-status,omitempty" yaml:"instance-status,omitempty"`
	PublicIPAddress   []string `json:"public-ip-address,omitempty" yaml:"public-ip-address,omitempty"`
	InternalIPAddress []string `json:"internal-ip-address,omitempty" yaml:"internal-ip-address,omitempty"`
	EipAllocationIds  []string `json:"eip-allocation-ids,omitempty" yaml:"eip-allocation-ids,omitempty"`
	RollBack          bool     `json:"-" yaml:"-"`
	Current           bool     `json:"-" yaml:"-"`
}

type SSH struct {
	Port             string `json:"ssh-port,omitempty" yaml:"ssh-port,omitempty"`
	User             string `json:"user,omitempty" yaml:"user,omitempty"`
	Password         string `json:"password,omitempty" yaml:"password,omitempty"`
	SSHKey           string `json:"ssh-key,omitempty" yaml:"ssh-key,omitempty"`
	SSHKeyPath       string `json:"ssh-key-path,omitempty" yaml:"ssh-key-path,omitempty"`
	SSHCert          string `json:"ssh-cert,omitempty" yaml:"ssh-cert,omitempty"`
	SSHCertPath      string `json:"ssh-cert-path,omitempty" yaml:"ssh-cert-path,omitempty"`
	SSHKeyPassphrase string `json:"ssh-key-passphrase,omitempty" yaml:"ssh-key-passphrase,omitempty"`
	SSHAgentAuth     bool   `json:"ssh-agent-auth,omitempty" yaml:"ssh-agent-auth,omitempty" `
}

type Flag struct {
	Name      string
	P         *string
	V         string
	ShortHand string
	Usage     string
	Required  bool
}
