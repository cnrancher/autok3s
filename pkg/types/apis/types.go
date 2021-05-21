package apis

import (
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/rancher/wrangler/pkg/schemas"
)

type Provider struct {
	Name    string                   `json:"name"`
	Options map[string]schemas.Field `json:"options,omitempty"`
	Config  map[string]schemas.Field `json:"config,omitempty"`
	Secrets map[string]schemas.Field `json:"secrets,omitempty"`
}

type Cluster struct {
	types.Metadata `json:",inline" mapstructure:",squash"`
	types.SSH      `json:",inline"`
	Options        interface{} `json:"options,omitempty"`
}

type Credential struct {
	ID       int               `json:"id"`
	Provider string            `json:"provider"`
	Secrets  map[string]string `json:"secrets,omitempty"`
}

type ProviderCredential struct {
	Provider     string                   `json:"provider"`
	SecretFields map[string]schemas.Field `json:"secretFields"`
}

type Mutual struct {
}

type Config struct {
	Context string `json:"context"`
}

type Logs struct {
}

type ClusterTemplate struct {
	types.Metadata `json:",inline" mapstructure:",squash"`
	types.SSH      `json:",inline"`
	Options        interface{} `json:"options,omitempty"`
	IsDefault      bool        `json:"is-default"`
	Status         string      `json:"status"`
}
