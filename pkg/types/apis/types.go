package apis

import (
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/rancher/wrangler/pkg/schemas"
)

// Provider struct for provider.
type Provider struct {
	Name    string                   `json:"name"`
	Options map[string]schemas.Field `json:"options,omitempty"`
	Config  map[string]schemas.Field `json:"config,omitempty"`
	Secrets map[string]schemas.Field `json:"secrets,omitempty"`
}

// Cluster struct for cluster.
type Cluster struct {
	types.Metadata `json:",inline" mapstructure:",squash"`
	types.SSH      `json:",inline"`
	Options        interface{} `json:"options,omitempty"`
}

// Credential struct for credential.
type Credential struct {
	ID       int               `json:"id"`
	Provider string            `json:"provider"`
	Secrets  map[string]string `json:"secrets,omitempty"`
}

// ProviderCredential struct for provider's credential.
type ProviderCredential struct {
	Provider     string                   `json:"provider"`
	SecretFields map[string]schemas.Field `json:"secretFields"`
}

// Mutual struct for mutual.
type Mutual struct {
}

// Config struct for config.
type Config struct {
	Context string `json:"context"`
}

// Logs struct for logs.
type Logs struct {
}

// ClusterTemplate struct for cluster template.
type ClusterTemplate struct {
	types.Metadata `json:",inline" mapstructure:",squash"`
	types.SSH      `json:",inline"`
	Options        interface{} `json:"options,omitempty"`
	IsDefault      bool        `json:"is-default"`
	Status         string      `json:"status"`
}
