package alibaba

var (
	StatusPending = "Pending"
	StatusRunning = "Running"
)

type Options struct {
	AccessKey               string            `json:"access-key,omitempty" yaml:"access-key,omitempty"`
	AccessSecret            string            `json:"access-secret,omitempty" yaml:"access-secret,omitempty"`
	DiskCategory            string            `json:"disk-category,omitempty" yaml:"disk-category,omitempty"`
	DiskSize                string            `json:"disk-size,omitempty" yaml:"disk-size,omitempty"`
	Image                   string            `json:"image,omitempty" yaml:"image,omitempty"`
	Terway                  string            `json:"terway,omitempty" yaml:"terway,omitempty"`
	TerwayMaxPoolSize       string            `json:"terway-max-pool-size,omitempty" yaml:"terway-max-pool-size,omitempty"`
	InstanceType            string            `json:"instance-type,omitempty" yaml:"instance-type,omitempty"`
	KeyPair                 string            `json:"key-pair,omitempty" yaml:"key-pair,omitempty"`
	Region                  string            `json:"region,omitempty" yaml:"region,omitempty"`
	Zone                    string            `json:"zone,omitempty" yaml:"zone,omitempty"`
	Vpc                     string            `json:"vpc,omitempty" yaml:"vpc,omitempty"`
	VSwitch                 string            `json:"v-switch,omitempty" yaml:"v-switch,omitempty"`
	SecurityGroup           string            `json:"security-group,omitempty" yaml:"security-group,omitempty"`
	InternetMaxBandwidthOut string            `json:"internet-max-bandwidth-out,omitempty" yaml:"internet-max-bandwidth-out,omitempty"`
	EIP                     bool              `json:"eip,omitempty" yaml:"eip,omitempty"`
	Tags                    map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	CloudControllerManager  bool              `json:"cloud-controller-manager" yaml:"cloud-controller-manager"`
}

type Terway struct {
	Mode          string `json:"mode,omitempty" yaml:"mode,omitempty"`
	AccessKey     string `json:"access-key,omitempty" yaml:"access-key,omitempty"`
	AccessSecret  string `json:"access-secret,omitempty" yaml:"access-secret,omitempty"`
	CIDR          string `json:"cidr,omitempty" yaml:"cidr,omitempty"`
	VSwitches     string `json:"v-switches,omitempty" yaml:"v-switches,omitempty"`
	SecurityGroup string `json:"security-group,omitempty" yaml:"security-group,omitempty"`
	MaxPoolSize   string `json:"max-pool-size,omitempty" yaml:"max-pool-size,omitempty"`
}

type CloudControllerManager struct {
	Region       string `json:"region,omitempty" yaml:"region,omitempty"`
	AccessKey    string `json:"access-key,omitempty" yaml:"access-key,omitempty"`
	AccessSecret string `json:"access-secret,omitempty" yaml:"access-secret,omitempty"`
}
