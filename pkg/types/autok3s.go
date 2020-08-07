package types

type AutoK3s struct {
	Clusters []Cluster `json:"clusters"`
}

type Cluster struct {
	Metadata `json:",inline" mapstructure:",squash"`

	Status `json:"status"`
}

type Metadata struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Master   string `json:"master"`
	Worker   string `json:"worker"`
}

type Status struct {
	MasterNodes []Node `json:"masterNodes,omitempty"`
	WorkerNodes []Node `json:"workerNodes,omitempty"`
}

type Node struct {
	SSH `json:",inline"`

	Master            bool     `json:"master,omitempty"`
	Port              string   `json:"port,omitempty"`
	InstanceID        string   `json:"instanceID,omitempty"`
	InstanceStatus    string   `json:"instanceStatus,omitempty"`
	PublicIPAddress   []string `json:"publicIPAddress,omitempty"`
	InternalIPAddress []string `json:"internalIPAddress,omitempty"`
}

type SSH struct {
	Port       string `json:"port,omitempty"`
	User       string `json:"user,omitempty"`
	SSHKey     string `json:"sshKey,omitempty"`
	SSHKeyPath string `json:"sshKeyPath,omitempty"`
}

type Flag struct {
	Name      string
	P         *string
	V         string
	ShortHand string
	Usage     string
	Required  bool
}
