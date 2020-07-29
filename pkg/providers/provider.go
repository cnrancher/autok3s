package providers

import (
	"github.com/Jason-ZW/autok3s/pkg/providers/alibaba"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type Provider interface {
	GetProviderName() string
	GetCreateFlags() *pflag.FlagSet
}

func Register(provider string) Provider {
	var p Provider

	switch provider {
	case "alibaba":
		p = alibaba.NewProvider()
	default:
		logrus.Fatalln("not a valid provider, please run `autok3s get provider` display valid providers")
	}

	return p
}

func SupportedProviders(provider string) [][]string {
	providers := [][]string{
		{"alibaba", "yes"},
	}
	if provider == "" {
		return providers
	}
	for _, ss := range providers {
		if ss[0] == provider {
			return [][]string{ss}
		}
	}

	return [][]string{}
}
