package providers

import (
	"fmt"
	"sync"

	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Factory is a function that returns a Provider.Interface.
type Factory func() (Provider, error)

var (
	providersMutex sync.Mutex
	providers      = make(map[string]Factory)
)

// Provider is an abstract, pluggable interface for k3s provider
type Provider interface {
	GetProviderName() string
	// Create command flags.
	GetCreateFlags(cmd *cobra.Command) *pflag.FlagSet
	// Join command flags.
	GetJoinFlags(cmd *cobra.Command) *pflag.FlagSet
	// Stop command flags.
	GetStopFlags(cmd *cobra.Command) *pflag.FlagSet
	// Start command flags.
	GetStartFlags(cmd *cobra.Command) *pflag.FlagSet
	// Delete command flags.
	GetDeleteFlags(cmd *cobra.Command) *pflag.FlagSet
	// Credential flags.
	GetCredentialFlags(cmd *cobra.Command) *pflag.FlagSet
	// Use this method to bind Viper, although it is somewhat repetitive.
	BindCredentialFlags() *pflag.FlagSet
	// Generate cluster name.
	GenerateClusterName()
	// Generate create/join extra args for master nodes
	GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string
	// Generate create/join extra args for worker nodes
	GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string
	// K3s create cluster interface.
	CreateK3sCluster(ssh *types.SSH) error
	// K3s join node interface.
	JoinK3sNode(ssh *types.SSH) error
	// K3s check cluster exist.
	IsClusterExist() (bool, []string, error)
	// Rollback when error occurs.
	Rollback() error
	// K3s delete node interface.
	DeleteK3sNode(f bool) error
	// K3s start cluster interface.
	StartK3sCluster() error
	// K3s stop cluster interface.
	StopK3sCluster(f bool) error
}

// RegisterProvider registers a provider.Factory by name.
func RegisterProvider(name string, p Factory) {
	providersMutex.Lock()
	defer providersMutex.Unlock()
	if _, found := providers[name]; !found {
		logrus.Debugf("registered provider %s", name)
		providers[name] = p
	}
}

// GetProvider creates an instance of the named provider, or nil if
// the name is unknown.  The error return is only used if the named provider
// was known but failed to initialize.
func GetProvider(name string) (Provider, error) {
	providersMutex.Lock()
	defer providersMutex.Unlock()
	f, found := providers[name]
	if !found {
		return nil, fmt.Errorf("provider %s is not registered", name)
	}
	return f()
}
