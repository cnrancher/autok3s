package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// AutoK3s struct for autok3s.
type AutoK3s struct {
	Clusters []Cluster `json:"clusters" yaml:"clusters"`
}

// Cluster struct for cluster.
type Cluster struct {
	Metadata `json:",inline" mapstructure:",squash"`
	Options  interface{} `json:"options,omitempty"`
	SSH      `json:",inline"`

	Status `json:"status" yaml:"status"`
}

// Metadata struct for metadata.
type Metadata struct {
	Name                     string      `json:"name" yaml:"name"`
	Provider                 string      `json:"provider" yaml:"provider"`
	Master                   string      `json:"master" yaml:"master"`
	Worker                   string      `json:"worker" yaml:"worker"`
	Token                    string      `json:"token,omitempty" yaml:"token,omitempty"`
	IP                       string      `json:"ip,omitempty" yaml:"ip,omitempty"`
	TLSSans                  StringArray `json:"tls-sans,omitempty" yaml:"tls-sans,omitempty" gorm:"type:text"`
	ClusterCidr              string      `json:"cluster-cidr,omitempty" yaml:"cluster-cidr,omitempty"`
	MasterExtraArgs          string      `json:"master-extra-args,omitempty" yaml:"master-extra-args,omitempty"`
	WorkerExtraArgs          string      `json:"worker-extra-args,omitempty" yaml:"worker-extra-args,omitempty"`
	Registry                 string      `json:"registry,omitempty" yaml:"registry,omitempty"`
	SystemDefaultRegistry    string      `json:"system-default-registry,omitempty" yaml:"system-default-registry,omitempty"`
	DataStore                string      `json:"datastore,omitempty" yaml:"datastore,omitempty"`
	K3sVersion               string      `json:"k3s-version,omitempty" yaml:"k3s-version,omitempty"`
	K3sChannel               string      `json:"k3s-channel,omitempty" yaml:"k3s-channel,omitempty"`
	InstallScript            string      `json:"k3s-install-script,omitempty" yaml:"k3s-install-script,omitempty"`
	Mirror                   string      `json:"k3s-install-mirror,omitempty" yaml:"k3s-install-mirror,omitempty"`
	DockerMirror             string      `json:"dockerMirror,omitempty" yaml:"dockerMirror,omitempty"`
	DockerArg                string      `json:"docker-arg,omitempty" yaml:"docker-arg,omitempty"`
	DockerScript             string      `json:"docker-script,omitempty" yaml:"docker-script,omitempty"`
	Network                  string      `json:"network,omitempty" yaml:"network,omitempty"`
	UI                       bool        `json:"ui" yaml:"ui" gorm:"type:bool"` // Deprecated
	Cluster                  bool        `json:"cluster" yaml:"cluster" gorm:"type:bool"`
	ContextName              string      `json:"context-name" yaml:"context-name"`
	RegistryContent          string      `json:"registry-content,omitempty" yaml:"registry-content,omitempty"`
	Manifests                string      `json:"manifests,omitempty" yaml:"manifests,omitempty"`
	Enable                   StringArray `json:"enable,omitempty" yaml:"enable,omitempty" gorm:"type:stringArray"`
	PackagePath              string      `json:"package-path,omitempty" yaml:"package-path,omitempty"`
	PackageName              string      `json:"package-name,omitempty" yaml:"package-name,omitempty"`
	DataStoreCAFile          string      `json:"datastore-cafile,omitempty" yaml:"datastore-cafile,omitempty"`
	DataStoreCertFile        string      `json:"datastore-certfile,omitempty" yaml:"datastore-certfile,omitempty"`
	DataStoreKeyFile         string      `json:"datastore-keyfile,omitempty" yaml:"datastore-keyfile,omitempty"`
	DataStoreCAFileContent   string      `json:"datastore-cafile-content,omitempty" yaml:"datastore-cafile-content,omitempty"`
	DataStoreCertFileContent string      `json:"datastore-certfile-content,omitempty" yaml:"datastore-certfile-content,omitempty"`
	DataStoreKeyFileContent  string      `json:"datastore-keyfile-content,omitempty" yaml:"datastore-keyfile-content,omitempty"`
	Rollback                 bool        `json:"rollback" yaml:"rollback" gorm:"type:bool"`
	Values                   StringMap   `json:"values,omitempty" yaml:"values,omitempty" gorm:"type:stringMap"`
	InstallEnv               StringMap   `json:"install-env,omitempty" yaml:"install-env,omitempty" gorm:"type:stringMap"`
	ServerConfigFileContent  string      `json:"server-config-file-content,omitempty" yaml:"server-config-file-content,omitempty"`
	ServerConfigFile         string      `json:"server-config-file,omitempty" yaml:"server-config-file,omitempty"`
	AgentConfigFileContent   string      `json:"agent-config-file-content,omitempty" yaml:"agent-config-file-content,omitempty"`
	AgentConfigFile          string      `json:"agent-config-file,omitempty" yaml:"agent-config-file,omitempty"`
}

// Status struct for status.
type Status struct {
	Status      string `json:"status,omitempty"`
	MasterNodes []Node `json:"master-nodes,omitempty"`
	WorkerNodes []Node `json:"worker-nodes,omitempty"`
	Standalone  bool   `json:"standalone"`
}

// Node struct for node.
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
	Standalone        bool     `json:"standalone"`

	LocalHostname string `json:"local-hostname,omitempty" yaml:"local-hostname,omitempty"`
}

// SSH struct for ssh.
type SSH struct {
	SSHPort          string `json:"ssh-port,omitempty" yaml:"ssh-port,omitempty" default:"22"`
	SSHUser          string `json:"ssh-user,omitempty" yaml:"ssh-user,omitempty"`
	SSHPassword      string `json:"ssh-password,omitempty" yaml:"ssh-password,omitempty"`
	SSHKeyPath       string `json:"ssh-key-path,omitempty" yaml:"ssh-key-path,omitempty"`
	SSHCertPath      string `json:"ssh-cert-path,omitempty" yaml:"ssh-cert-path,omitempty"`
	SSHKeyPassphrase string `json:"ssh-key-passphrase,omitempty" yaml:"ssh-key-passphrase,omitempty"`
	SSHAgentAuth     bool   `json:"ssh-agent-auth,omitempty" yaml:"ssh-agent-auth,omitempty"`

	SSHKeyName string `json:"ssh-key-name,omitempty" yaml:"ssh-key-name,omitempty" norman:"type=reference[sshkey]"`
	SSHKey     string `json:"ssh-key,omitempty" yaml:"ssh-key,omitempty" norman:"type=password" gorm:"-:all"`
	// SSHCert is no longer needed in db, the content will be read and saved to the cluster's directory.
	// There was no way to set the SSHCert so it is save to ignore the value in DB.
	SSHCert string `json:"ssh-cert,omitempty" yaml:"ssh-cert,omitempty" gorm:"-:all"`
}

// Flag struct for flag.
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
	// ClusterStatusRunning cluster running status.
	ClusterStatusRunning = "Running"
	// ClusterStatusStopped cluster stopped status.
	ClusterStatusStopped = "Stopped"
	// ClusterStatusUnknown cluster unknown status.
	ClusterStatusUnknown = "Unknown"
)

// ClusterInfo struct for cluster info.
type ClusterInfo struct {
	ID            string        `json:"id,omitempty"`
	Name          string        `json:"name,omitempty"`
	Region        string        `json:"region,omitempty"`
	Zone          string        `json:"zone,omitempty"`
	Provider      string        `json:"provider,omitempty"`
	Status        string        `json:"status,omitempty"`
	Master        string        `json:"master,omitempty"`
	Worker        string        `json:"worker,omitempty"`
	Version       string        `json:"version,omitempty"`
	Nodes         []ClusterNode `json:"nodes,omitempty"`
	IsHAMode      bool          `json:"is-ha-mode,omitempty"`
	DataStoreType string        `json:"datastore-type,omitempty"`
}

// ClusterNode struct for cluster node.
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
	Standalone              bool     `json:"standalone"`
}

// StringArray gorm custom string array flag type.
type StringArray []string

// Scan gorm Scan implement.
func (a *StringArray) Scan(value interface{}) (err error) {
	switch v := value.(type) {
	case string:
		if v != "" {
			*a = strings.Split(v, ",")
		}
	default:
		return fmt.Errorf("failed to scan array value %v", value)
	}
	return nil
}

// Value gorm Value implement.
func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return nil, nil
	}
	return strings.Join(a, ","), nil
}

// GormDataType returns gorm data type.
func (a StringArray) GormDataType() string {
	return "string"
}

func (a StringArray) Contains(target string) bool {
	for _, content := range a {
		if target == content {
			return true
		}
	}
	return false
}

func (m *Metadata) GetID() string {
	return m.ContextName
}

type StringMap map[string]string

func (ss *StringMap) Scan(value interface{}) (err error) {
	var ba []byte
	switch v := value.(type) {
	case string:
		ba = []byte(v)
	case []byte:
		ba = v
	default:
		return fmt.Errorf("failed to scan value %v", value)
	}
	t := map[string]string{}
	err = json.Unmarshal(ba, &t)
	*ss = t
	return err
}

func (ss StringMap) Value() (driver.Value, error) {
	if ss == nil {
		return nil, nil
	}
	ba, err := json.Marshal(ss)
	return string(ba), err
}

func (ss StringMap) GormDataType() string {
	return "bytes"
}
