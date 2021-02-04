package apis

import (
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas"
)

type Provider struct {
	Name    string                   `json:"name"`
	Options map[string]schemas.Field `json:"options,omitempty"`
	Config  map[string]schemas.Field `json:"config,omitempty"`
}

type Cluster struct {
	types.Metadata `json:",inline" mapstructure:",squash"`
	types.SSH      `json:",inline"`
	Options        interface{} `json:"options,omitempty"`
}

type Credential struct {
	Provider     string                   `json:"provider"`
	SecretFields map[string]schemas.Field `json:"secretFields"`
	Secrets      map[string]string        `json:"secrets,omitempty"`
}

type Mutual struct {
}

type Config struct {
	Context string `json:"context"`
}

type Logs struct {
}
