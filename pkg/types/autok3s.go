package types

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

type AutoK3s struct {
	Clusters []Cluster `json:"clusters" yaml:"clusters"`
}

type Cluster struct {
	Metadata `json:",inline" mapstructure:",squash"`
	Options  interface{} `json:"options,omitempty"`
	SSH      `json:",inline"`

	Status `json:"status" yaml:"status"`
}

type Metadata struct {
	Name            string      `json:"name" yaml:"name"`
	Provider        string      `json:"provider" yaml:"provider"`
	Master          string      `json:"master" yaml:"master"`
	Worker          string      `json:"worker" yaml:"worker"`
	Token           string      `json:"token,omitempty" yaml:"token,omitempty"`
	IP              string      `json:"ip,omitempty" yaml:"ip,omitempty"`
	TLSSans         StringArray `json:"tls-sans,omitempty" yaml:"tls-sans,omitempty" gorm:"type:stringArray"`
	ClusterCidr     string      `json:"cluster-cidr,omitempty" yaml:"cluster-cidr,omitempty"`
	MasterExtraArgs string      `json:"master-extra-args,omitempty" yaml:"master-extra-args,omitempty"`
	WorkerExtraArgs string      `json:"worker-extra-args,omitempty" yaml:"worker-extra-args,omitempty"`
	Registry        string      `json:"registry,omitempty" yaml:"registry,omitempty"`
	DataStore       string      `json:"datastore,omitempty" yaml:"datastore,omitempty"`
	K3sVersion      string      `json:"k3s-version,omitempty" yaml:"k3s-version,omitempty"`
	K3sChannel      string      `json:"k3s-channel,omitempty" yaml:"k3s-channel,omitempty"`
	InstallScript   string      `json:"k3s-install-script,omitempty" yaml:"k3s-install-script,omitempty"`
	Mirror          string      `json:"k3s-install-mirror,omitempty" yaml:"k3s-install-mirror,omitempty"`
	DockerMirror    string      `json:"dockerMirror,omitempty" yaml:"dockerMirror,omitempty"`
	DockerScript    string      `json:"dockerScript,omitempty" yaml:"dockerScript,omitempty"`
	Network         string      `json:"network,omitempty" yaml:"network,omitempty"`
	UI              bool        `json:"ui" yaml:"ui"`
	Cluster         bool        `json:"cluster" yaml:"cluster"`
	ContextName     string      `json:"context-name" yaml:"context-name"`
	RegistryContent string      `json:"registry-content,omitempty" yaml:"registry-content,omitempty"`
	Manifests       string      `json:"manifests,omitempty" yaml:"manifests,omitempty"`
}

type Status struct {
	Status      string `json:"status,omitempty"`
	MasterNodes []Node `json:"master-nodes,omitempty"`
	WorkerNodes []Node `json:"worker-nodes,omitempty"`
}

type Node struct {
	SSH `json:",inline"`

	InstanceID        string   `json:"instance-id,omitempty" yaml:"instance-id,omitempty"`
	InstanceStatus    string   `json:"instance-status,omitempty" yaml:"instance-status,omitempty"`
	PublicIPAddress   []string `json:"public-ip-address,omitempty" yaml:"public-ip-address,omitempty"`
	InternalIPAddress []string `json:"internal-ip-address,omitempty" yaml:"internal-ip-address,omitempty"`
	EipAllocationIds  []string `json:"eip-allocation-ids,omitempty" yaml:"eip-allocation-ids,omitempty"`
	Master            bool     `json:"master,omitempty" yaml:"master,omitempty"`
	RollBack          bool     `json:"-" yaml:"-"`
	Current           bool     `json:"-" yaml:"-"`
}

type SSH struct {
	SSHPort          string `json:"ssh-port,omitempty" yaml:"ssh-port,omitempty" default:"22"`
	SSHUser          string `json:"ssh-user,omitempty" yaml:"ssh-user,omitempty"`
	SSHPassword      string `json:"ssh-password,omitempty" yaml:"ssh-password,omitempty"`
	SSHKeyPath       string `json:"ssh-key-path,omitempty" yaml:"ssh-key-path,omitempty"`
	SSHCert          string `json:"ssh-cert,omitempty" yaml:"ssh-cert,omitempty"`
	SSHCertPath      string `json:"ssh-cert-path,omitempty" yaml:"ssh-cert-path,omitempty"`
	SSHKeyPassphrase string `json:"ssh-key-passphrase,omitempty" yaml:"ssh-key-passphrase,omitempty"`
	SSHAgentAuth     bool   `json:"ssh-agent-auth,omitempty" yaml:"ssh-agent-auth,omitempty" `
}

type Flag struct {
	Name      string
	P         interface{}
	V         interface{}
	ShortHand string
	Usage     string
	Required  bool
	EnvVar    string
}

const (
	ClusterStatusRunning = "Running"
	ClusterStatusStopped = "Stopped"
	ClusterStatusUnknown = "Unknown"
)

type ClusterInfo struct {
	ID       string        `json:"id,omitempty"`
	Name     string        `json:"name,omitempty"`
	Region   string        `json:"region,omitempty"`
	Zone     string        `json:"zone,omitempty"`
	Provider string        `json:"provider,omitempty"`
	Status   string        `json:"status,omitempty"`
	Master   string        `json:"master,omitempty"`
	Worker   string        `json:"worker,omitempty"`
	Version  string        `json:"version,omitempty"`
	Nodes    []ClusterNode `json:"nodes,omitempty"`
}

type ClusterNode struct {
	InstanceID              string   `json:"instance-id,omitempty"`
	InstanceStatus          string   `json:"instance-status,omitempty"`
	ExternalIP              []string `json:"external-ip,omitempty"`
	InternalIP              []string `json:"internal-ip,omitempty"`
	Roles                   string   `json:"roles,omitempty"`
	Status                  string   `json:"status,omitempty"`
	HostName                string   `json:"hostname,omitempty"`
	ContainerRuntimeVersion string   `json:"containerRuntimeVersion,omitempty"`
	Version                 string   `json:"version,omitempty"`
	Master                  bool     `json:"-"`
}

type StringArray []string

func (a *StringArray) Scan(value interface{}) (err error) {
	switch v := value.(type) {
	case string:
		*a = strings.Split(v, ",")
	default:
		return fmt.Errorf("failed to scan array value %v", value)
	}
	return nil
}

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return strings.Join(a, ","), nil
}

func (a StringArray) GormDataType() string {
	return "stringArray"
}
