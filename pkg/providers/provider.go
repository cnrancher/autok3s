package providers

import (
	"fmt"
	"sync"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/sirupsen/logrus"
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
	// Get command usage example.
	GetUsageExample(action string) string
	// create flags
	GetCreateFlags() []types.Flag
	// Create flags of provider options.
	GetOptionFlags() []types.Flag
	// Join command flags.
	GetJoinFlags() []types.Flag
	// Delete command flags.
	GetDeleteFlags() []types.Flag
	// SSH command flags.
	GetSSHFlags() []types.Flag
	// Credential flags.
	GetCredentialFlags() []types.Flag
	// Generate cluster name.
	GenerateClusterName() string
	// create/join extra master args for different provider
	GenerateMasterExtraArgs(cluster *types.Cluster, master types.Node) string
	// create/join extra worker args for different provider
	GenerateWorkerExtraArgs(cluster *types.Cluster, worker types.Node) string
	// K3s create cluster interface.
	CreateK3sCluster() error
	// K3s join node interface.
	JoinK3sNode() error
	// K3s delete cluster interface.
	DeleteK3sCluster(f bool) error
	// K3s ssh node interface.
	SSHK3sNode(node string) error
	// K3s check cluster exist.
	IsClusterExist() (bool, []string, error)
	// merge exist cluster options
	MergeClusterOptions() error
	// describe detailed cluster information
	DescribeCluster(kubecfg string) *types.ClusterInfo
	// get cluster simple information
	GetCluster(kubecfg string) *types.ClusterInfo
	// get default ssh config for provider
	GetSSHConfig() *types.SSH
	// set cluster configuration of provider
	SetConfig(config []byte) error
	// validate create flags
	CreateCheck() error
	// merge metadata configs for provider
	SetMetadata(config *types.Metadata)
	// merge provider options
	SetOptions(opt []byte) error
	// validate join flags
	JoinCheck() error
	// get cluster config options
	GetClusterOptions() []types.Flag
	// get create command options
	GetCreateOptions() []types.Flag
	// convert options to specified provider option interface
	GetProviderOptions(opt []byte) (interface{}, error)
	// persistent credential from flags to db
	BindCredential() error
	// callback functions used for execute logic after create/join
	RegisterCallbacks(name, event string, fn func(interface{}))
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

func ListProviders() []apis.Provider {
	providersMutex.Lock()
	defer providersMutex.Unlock()
	list := make([]apis.Provider, 0)
	for p := range providers {
		list = append(list, apis.Provider{
			Name: p,
		})
	}
	return list
}
