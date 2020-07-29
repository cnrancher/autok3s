package alibaba

import (
	"github.com/spf13/pflag"
)

type Alibaba struct {
	*Options
}

type Options struct {
	AccessKeyID string
	AccessKeySecret string
	Region string
}

func NewProvider() *Alibaba {
	return &Alibaba{&Options{}}
}

func (p *Alibaba) GetProviderName() string {
	return "alibaba"
}

func (p *Alibaba) GetCreateFlags() *pflag.FlagSet {
	nfs := pflag.NewFlagSet("Additional Options", pflag.ContinueOnError)
	nfs.StringVar(&p.AccessKeyID, "access-key-id", p.AccessKeyID, "user access key id")
	nfs.StringVar(&p.AccessKeySecret, "access-key-secret", p.AccessKeySecret, "user access key secret")
	nfs.StringVarP(&p.Region, "region", "r", p.Region, "regions are physical locations (data centers) that spread all over the world to reduce the network latency")
	return nfs
}
