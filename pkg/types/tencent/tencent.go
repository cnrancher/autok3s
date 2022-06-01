package tencent

const (
	// StatusPending tencent instance pending status.
	StatusPending = "PENDING"
	// StatusRunning tencent instance running status.
	StatusRunning = "RUNNING"

	// Success tencent task success result.
	Success = "SUCCESS"
	// Failed tencent task failed result.
	Failed = "FAILED"
	// Running tencent task running result.
	Running = "RUNNING"

	// ServiceTypeEIP eip service type.
	ServiceTypeEIP = "cvm"
	// ResourcePrefixEIP eip resource prefix.
	ResourcePrefixEIP = "eip"
)

// Options tencent provider's custom parameters.
type Options struct {
	SecretID                string   `json:"secret-id,omitempty" yaml:"secret-id,omitempty"`
	SecretKey               string   `json:"secret-key,omitempty" yaml:"secret-key,omitempty"`
	Region                  string   `json:"region,omitempty" yaml:"region,omitempty"`
	Zone                    string   `json:"zone,omitempty" yaml:"zone,omitempty"`
	EndpointURL             string   `json:"endpoint-url,omitempty" yaml:"endpoint-url,omitempty"`
	SecurityGroupIds        string   `json:"security-group,omitempty" yaml:"security-group,omitempty"`
	KeypairID               string   `json:"keypair-id,omitempty" yaml:"keypair-id,omitempty"`
	VpcID                   string   `json:"vpc,omitempty" yaml:"vpc,omitempty"`
	SubnetID                string   `json:"subnet,omitempty" yaml:"subnet,omitempty"`
	ImageID                 string   `json:"image,omitempty" yaml:"image,omitempty"`
	InstanceType            string   `json:"instance-type,omitempty" yaml:"instance-type,omitempty"`
	InstanceChargeType      string   `json:"instance-charge-type,omitempty" yaml:"instance-charge-type,omitempty"`
	SystemDiskType          string   `json:"disk-category,omitempty" yaml:"disk-category,omitempty"`
	SystemDiskSize          string   `json:"disk-size,omitempty" yaml:"disk-size,omitempty"`
	InternetMaxBandwidthOut string   `json:"internet-max-bandwidth-out,omitempty" yaml:"internet-max-bandwidth-out,omitempty"`
	PublicIPAssignedEIP     bool     `json:"eip" yaml:"eip"`
	NetworkRouteTableName   string   `json:"router,omitempty" yaml:"network-route-table-name,omitempty"`
	Tags                    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	CloudControllerManager  bool     `json:"cloud-controller-manager" yaml:"cloud-controller-manager"`
	UserDataPath            string   `json:"user-data-path,omitempty" yaml:"user-data-path,omitempty"`
	UserDataContent         string   `json:"user-data-content,omitempty" yaml:"user-data-content,omitempty"`
}

// CloudControllerManager struct for tencent cloud-controller-manager.
type CloudControllerManager struct {
	SecretID              string `json:"secret-id,omitempty" yaml:"secret-id,omitempty"`
	SecretKey             string `json:"secret-key,omitempty" yaml:"secret-key,omitempty"`
	Region                string `json:"region,omitempty" yaml:"region,omitempty"`
	VpcID                 string `json:"vpc-id,omitempty" yaml:"vpc-id,omitempty"`
	NetworkRouteTableName string `json:"router,omitempty" yaml:"router,omitempty"`
}
