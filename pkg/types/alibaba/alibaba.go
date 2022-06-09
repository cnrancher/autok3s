package alibaba

var (
	// StatusPending alibaba instance pending status.
	StatusPending = "Pending"
	// StatusRunning alibaba instance running status.
	StatusRunning = "Running"
)

// Options alibaba provider's custom parameters.
type Options struct {
	AccessKey               string   `json:"access-key,omitempty" yaml:"access-key,omitempty"`
	AccessSecret            string   `json:"access-secret,omitempty" yaml:"access-secret,omitempty"`
	DiskCategory            string   `json:"disk-category,omitempty" yaml:"disk-category,omitempty"`
	DiskSize                string   `json:"disk-size,omitempty" yaml:"disk-size,omitempty"`
	Image                   string   `json:"image,omitempty" yaml:"image,omitempty"`
	Terway                  string   `json:"terway,omitempty" yaml:"terway,omitempty"`
	TerwayMaxPoolSize       string   `json:"terway-max-pool-size,omitempty" yaml:"terway-max-pool-size,omitempty"`
	InstanceType            string   `json:"instance-type,omitempty" yaml:"instance-type,omitempty"`
	KeyPair                 string   `json:"key-pair,omitempty" yaml:"key-pair,omitempty"`
	Region                  string   `json:"region,omitempty" yaml:"region,omitempty"`
	Zone                    string   `json:"zone,omitempty" yaml:"zone,omitempty"`
	Vpc                     string   `json:"vpc,omitempty" yaml:"vpc,omitempty"`
	VSwitch                 string   `json:"v-switch,omitempty" yaml:"v-switch,omitempty"`
	SecurityGroup           string   `json:"security-group,omitempty" yaml:"security-group,omitempty"`
	InternetMaxBandwidthOut string   `json:"internet-max-bandwidth-out,omitempty" yaml:"internet-max-bandwidth-out,omitempty"`
	EIP                     bool     `json:"eip,omitempty" yaml:"eip,omitempty"`
	Tags                    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	CloudControllerManager  bool     `json:"cloud-controller-manager" yaml:"cloud-controller-manager"`
	UserDataPath            string   `json:"user-data-path,omitempty" yaml:"user-data-path,omitempty"`
	UserDataContent         string   `json:"user-data-content,omitempty" yaml:"user-data-content,omitempty"`
	SpotStrategy            string   `json:"spot-strategy,omitempty" yaml:"spot-strategy,omitempty"`
	SpotDuration            int      `json:"spot-duration,omitempty" yaml:"spot-duration,omitempty"`
	SpotPriceLimit          float64  `json:"spot-price-limit,omitempty" yaml:"spot-price-limit,omitempty"`
}

// Terway struct for alibaba terway.
type Terway struct {
	Mode          string `json:"mode,omitempty" yaml:"mode,omitempty"`
	AccessKey     string `json:"access-key,omitempty" yaml:"access-key,omitempty"`
	AccessSecret  string `json:"access-secret,omitempty" yaml:"access-secret,omitempty"`
	CIDR          string `json:"cidr,omitempty" yaml:"cidr,omitempty"`
	VSwitches     string `json:"v-switches,omitempty" yaml:"v-switches,omitempty"`
	SecurityGroup string `json:"security-group,omitempty" yaml:"security-group,omitempty"`
	MaxPoolSize   string `json:"max-pool-size,omitempty" yaml:"max-pool-size,omitempty"`
}

// CloudControllerManager struct for alibaba cloud-controller-manager.
type CloudControllerManager struct {
	Region       string `json:"region,omitempty" yaml:"region,omitempty"`
	AccessKey    string `json:"access-key,omitempty" yaml:"access-key,omitempty"`
	AccessSecret string `json:"access-secret,omitempty" yaml:"access-secret,omitempty"`
}
