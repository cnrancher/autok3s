package alibaba

var (
	StatusPending = "Pending"
	StatusRunning = "Running"
)

type Options struct {
	AccessKeyID             string `json:"accessKeyID,omitempty"`
	AccessKeySecret         string `json:"accessKeySecret,omitempty"`
	DiskCategory            string `json:"diskCategory,omitempty"`
	DiskSize                string `json:"diskSize,omitempty"`
	ImageID                 string `json:"imageID,omitempty"`
	InstanceType            string `json:"instanceType,omitempty"`
	KeyPairName             string `json:"keyPairName,omitempty"`
	Region                  string `json:"region,omitempty"`
	VSwitchID               string `json:"vSwitchID,omitempty"`
	SecurityGroupID         string `json:"securityGroupID,omitempty"`
	InternetMaxBandwidthOut string `json:"internetMaxBandwidthOut,omitempty"`
}
