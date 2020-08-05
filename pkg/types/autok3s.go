package types

import "github.com/Jason-ZW/autok3s/pkg/types/alibaba"

type AutoK3s struct {
	Clusters []Cluster `json:"clusters"`
}

type Cluster struct {
	Metadata `json:",inline" mapstructure:",squash"`

	alibaba.Options `json:",inline" mapstructure:",squash"`
}

type Metadata struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Master   int    `json:"master"`
	Worker   int    `json:"worker"`
}
