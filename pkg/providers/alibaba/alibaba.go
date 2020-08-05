package alibaba

import (
	"github.com/Jason-ZW/autok3s/pkg/types"
	"github.com/Jason-ZW/autok3s/pkg/types/alibaba"
	pkgviper "github.com/Jason-ZW/autok3s/pkg/viper"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type Alibaba struct {
	types.Metadata `json:",inline"`

	alibaba.Options `json:",inline"`

	ECSClient *ecs.Client
}

func NewProvider() *Alibaba {
	return &Alibaba{
		Metadata: types.Metadata{},
		Options:  alibaba.Options{},
	}
}

func (p *Alibaba) GetProviderName() string {
	return "alibaba"
}

func (p *Alibaba) GetCreateFlags() *pflag.FlagSet {
	nfs := pflag.NewFlagSet("", pflag.ContinueOnError)
	nfs.StringVarP(&p.Name, "name", "n", p.Name, "Cluster name")
	nfs.StringVarP(&p.Region, "region", "r", p.Region, "Regions are physical locations (data centers) that spread all over the world to reduce the network latency")
	return nfs
}

func (p *Alibaba) GetCredentialFlags() *pflag.FlagSet {
	nfs := pflag.NewFlagSet("", pflag.ContinueOnError)
	nfs.StringVar(&p.AccessKeyID, "accessKeyID", p.AccessKeyID, "User access key ID")
	nfs.StringVar(&p.AccessKeySecret, "accessKeySecret", p.AccessKeySecret, "User access key secret")
	return nfs
}

func (p *Alibaba) CreateK3sCluster() {
	p.generateClientSDK()
}

func (p *Alibaba) generateClientSDK() {
	client, err := ecs.NewClientWithAccessKey(p.Region, pkgviper.GetString(p.GetProviderName(), "accessKeyID"),
		pkgviper.GetString(p.GetProviderName(), "accessKeySecret"))
	if err != nil {
		logrus.Fatalln(err)
	}
	p.ECSClient = client
}
