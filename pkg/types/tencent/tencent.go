package tencent

const (
	StatusPending = "PENDING"
	StatusRunning = "RUNNING"
	StatusStopped = "STOPPED"

	// task result
	Success = "SUCCESS"
	Failed  = "FAILED"
	Running = "RUNNING"

	// filter eip
	ServiceTypeEIP    = "cvm"
	ResourcePrefixEIP = "eip"
)

type Options struct {
	SecretID                string `json:"secret-id,omitempty" yaml:"secret-id,omitempty"`
	SecretKey               string `json:"secret-key,omitempty" yaml:"secret-key,omitempty"`
	Region                  string `json:"region,omitempty" yaml:"region,omitempty"`
	Zone                    string `json:"zone,omitempty" yaml:"zone,omitempty"`
	EndpointURL             string `json:"endpoint-url,omitempty" yaml:"endpoint-url,omitempty"`
	SecurityGroupIds        string `json:"security-group-ids,omitempty" yaml:"security-group-ids,omitempty"`
	KeyIds                  string `json:"key-ids,omitempty" yaml:"key-ids,omitempty"`
	VpcID                   string `json:"vpc-id,omitempty" yaml:"vpc-id,omitempty"`
	SubnetID                string `json:"subnet-id,omitempty" yaml:"subnet-id,omitempty"`
	ImageID                 string `json:"image-id,omitempty" yaml:"image-id,omitempty"`
	InstanceType            string `json:"instance-type,omitempty" yaml:"instance-type,omitempty"`
	SystemDiskType          string `json:"system-disk-type,omitempty" yaml:"system-disk-type,omitempty"`
	SystemDiskSize          string `json:"system-disk-size,omitempty" yaml:"system-disk-size,omitempty"`
	InternetMaxBandwidthOut string `json:"internet-max-bandwidth-out,omitempty" yaml:"internet-max-bandwidth-out,omitempty"`
	PublicIPAssignedEIP     bool   `json:"public-ip-assigned-eip" yaml:"public-ip-assigned-eip"`
	NetworkRouteTableName   string `json:"network-route-table-name,omitempty" yaml:"network-route-table-name,omitempty"`
}

type CloudControllerManager struct {
	SecretID              string `json:"secret-id,omitempty" yaml:"secret-id,omitempty"`
	SecretKey             string `json:"secret-key,omitempty" yaml:"secret-key,omitempty"`
	Region                string `json:"region,omitempty" yaml:"region,omitempty"`
	VpcID                 string `json:"vpc-id,omitempty" yaml:"vpc-id,omitempty"`
	NetworkRouteTableName string `json:"network-route-table-name,omitempty" yaml:"network-route-table-name,omitempty"`
}
