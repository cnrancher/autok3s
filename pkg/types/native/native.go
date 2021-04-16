package native

type Options struct {
	MasterIps string `json:"master-ips,omitempty" yaml:"master-ips,omitempty"`
	WorkerIps string `json:"worker-ips,omitempty" yaml:"worker-ips,omitempty"`
}
