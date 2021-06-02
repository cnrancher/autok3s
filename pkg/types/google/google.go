package google

type Options struct {
	Region                 string   `json:"region,omitempty" yaml:"region,omitempty"`
	Zone                   string   `json:"zone,omitempty" yaml:"zone,omitempty"`
	MachineType            string   `json:"machine-type,omitempty" yaml:"machine-type,omitempty"`
	MachineImage           string   `json:"machine-image,omitempty" yaml:"machine-image,omitempty"`
	DiskType               string   `json:"disk-type,omitempty" yaml:"disk-type,omitempty"`
	VMNetwork              string   `json:"vm-network,omitempty" yaml:"vm-network,omitempty"`
	Subnetwork             string   `json:"subnetwork,omitempty" yaml:"subnetwork,omitempty"`
	Preemptible            bool     `json:"preemptible" yaml:"preemptible"`
	UseInternalIPOnly      bool     `json:"use-internal-ip-only" yaml:"use-internal-ip-only"`
	ServiceAccount         string   `json:"service-account" yaml:"service-account"`
	ServiceAccountFile     string   `json:"service-account-file" yaml:"service-account-file"`
	Scopes                 string   `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	DiskSize               int      `json:"disk-size,omitempty" yaml:"disk-size,omitempty"`
	Project                string   `json:"project" yaml:"project"`
	Tags                   []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	OpenPorts              []string `json:"open-ports,omitempty" yaml:"open-ports,omitempty"`
	CloudControllerManager bool     `json:"cloud-controller-manager" yaml:"cloud-controller-manager"`
}
