package k3d

// Options k3d provider's custom parameters.
type Options struct {
	APIPort       string   `json:"api-port,omitempty" yaml:"api-port,omitempty"`
	Envs          []string `json:"envs,omitempty" yaml:"envs,omitempty"`
	GPUs          string   `json:"gpus,omitempty" yaml:"gpus,omitempty"`
	Image         string   `json:"image,omitempty" yaml:"image,omitempty"`
	Labels        []string `json:"labels,omitempty" yaml:"labels,omitempty"`
	MastersMemory string   `json:"masters-memory,omitempty" yaml:"masters-memory,omitempty"`
	NoLB          bool     `json:"no-lb,omitempty" yaml:"no-lb,omitempty"`
	NoImageVolume bool     `json:"no-image-volume,omitempty" yaml:"no-image-volume,omitempty"`
	Ports         []string `json:"ports,omitempty" yaml:"ports,omitempty"`
	Volumes       []string `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WorkersMemory string   `json:"workers-memory,omitempty" yaml:"workers-memory,omitempty"`
}
