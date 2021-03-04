package aws

type Options struct {
	AccessKey                    string            `json:"access-key,omitempty" yaml:"access-key,omitempty"`
	SecretKey                    string            `json:"secret-key,omitempty" yaml:"secret-key,omitempty"`
	Region                       string            `json:"region,omitempty" yaml:"region,omitempty"`
	AMI                          string            `json:"ami,omitempty" yaml:"ami,omitempty"`
	KeypairName                  string            `json:"keypair-name,omitempty" yaml:"keypair-name,omitempty"`
	InstanceType                 string            `json:"instance-type,omitempty" yaml:"instance-type,omitempty"`
	SecurityGroup                string            `json:"security-group,omitempty" yaml:"security-group,omitempty"`
	RootSize                     string            `json:"root-size,omitempty" yaml:"root-size,omitempty"`
	VolumeType                   string            `json:"volume-type,omitempty" yaml:"volume-type,omitempty"`
	VpcID                        string            `json:"vpc-id,omitempty" yaml:"vpc-id,omitempty"`
	SubnetID                     string            `json:"subnet-id,omitempty" yaml:"subnet-id,omitempty"`
	Zone                         string            `json:"zone,omitempty" yaml:"zone,omitempty"`
	IamInstanceProfileForControl string            `json:"iam-instance-profile-control,omitempty" yaml:"iam-instance-profile-control,omitempty"`
	IamInstanceProfileForWorker  string            `json:"iam-instance-profile-worker,omitempty" yaml:"iam-instance-profile-worker,omitempty"`
	RequestSpotInstance          bool              `json:"request-spot-instance,omitempty" yaml:"request-spot-instance,omitempty"`
	SpotPrice                    string            `json:"spot-price,omitempty" yaml:"spot-price,omitempty"`
	Tags                         map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}
