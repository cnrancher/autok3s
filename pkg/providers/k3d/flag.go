package k3d

import (
	"reflect"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/k3d"
	"github.com/cnrancher/autok3s/pkg/utils"
)

const createUsageExample = `  autok3s -d create \
    --provider k3d \
    --name <cluster name> \
    --master 1
`

const joinUsageExample = `  autok3s -d join \
    --provider k3d \
    --name <cluster name> \
    --worker 1
`

const deleteUsageExample = `  autok3s -d delete \
    --provider k3d \
    --name <cluster name>
`

const sshUsageExample = `  autok3s ssh \
    --provider k3d \
    --name <cluster name>
`

func (p *K3d) GetUsageExample(action string) string {
	switch action {
	case "create":
		return createUsageExample
	case "join":
		return joinUsageExample
	case "delete":
		return deleteUsageExample
	case "ssh":
		return sshUsageExample
	default:
		return ""
	}
}

func (p *K3d) GetCreateFlags() []types.Flag {
	fs := p.sharedFlags()
	fs = append(fs, []types.Flag{
		{
			Name:  "token",
			P:     &p.Token,
			V:     p.Token,
			Usage: "K3s token, if empty will automatically generated, see: https://rancher.com/docs/k3s/latest/en/installation/install-options/server-config/#cluster-options",
		},
		{
			Name:     "network",
			P:        &p.Network,
			V:        p.Network,
			Usage:    "Join an existing network, see: https://k3d.io/internals/networking",
			Required: false,
		},
	}...)
	return fs
}

func (p *K3d) GetSSHConfig() *types.SSH {
	return &types.SSH{}
}

func (p *K3d) GetOptionFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:     "api-port",
			P:        &p.APIPort,
			V:        p.APIPort,
			Usage:    "Specify the Kubernetes API server port exposed on the LoadBalancer, e.g.(--api-port 0.0.0.0:6550)",
			Required: false,
		},
		{
			Name:     "ports",
			P:        &p.Ports,
			V:        p.Ports,
			Usage:    "Map ports from the node containers to the host, e.g.(--ports 8080:80@agent[0] --ports 8081@agent[1])",
			Required: false,
		},
		{
			Name:     "envs",
			P:        &p.Envs,
			V:        p.Envs,
			Usage:    "Add environment variables to nodes, e.g.(--envs HTTP_PROXY=my.proxy.com@server[0] --envs SOME_KEY=SOME_VAL@server[0])",
			Required: false,
		},
		{
			Name:     "volumes",
			P:        &p.Volumes,
			V:        p.Volumes,
			Usage:    "Mount volumes into the nodes, e.g.(--volumes /my/path@agent[0,1] --volumes /tmp/test:/tmp/other@server[0])",
			Required: false,
		},
		{
			Name:     "labels",
			P:        &p.Labels,
			V:        p.Labels,
			Usage:    "Add label to node container, e.g.(--labels my.label@agent[0,1] --labels other.label=somevalue@server[0])",
			Required: false,
		},
		{
			Name:     "gpus",
			P:        &p.GPUs,
			V:        p.GPUs,
			Usage:    "GPU devices to add to the cluster node containers ('all' to pass all GPUs) [From docker]",
			Required: false,
		},
		{
			Name:     "no-lb",
			P:        &p.NoLB,
			V:        p.NoLB,
			Usage:    "Disable the creation of a LoadBalancer in front of the server nodes",
			Required: false,
		},
		{
			Name:     "no-hostip",
			P:        &p.NoHostIP,
			V:        p.NoHostIP,
			Usage:    "Disable the automatic injection of the Host IP as 'host.k3d.internal' into the containers and CoreDNS",
			Required: false,
		},
		{
			Name:     "no-image-volume",
			P:        &p.NoImageVolume,
			V:        p.NoImageVolume,
			Usage:    "Disable the creation of a volume for importing images",
			Required: false,
		},
		{
			Name:     "masters-memory",
			P:        &p.MastersMemory,
			V:        p.MastersMemory,
			Usage:    "Memory limit imposed on the server nodes [From docker]",
			Required: false,
		},
		{
			Name:     "workers-memory",
			P:        &p.WorkersMemory,
			V:        p.WorkersMemory,
			Usage:    "Memory limit imposed on the agents nodes [From docker]",
			Required: false,
		},
		{
			Name:     "image",
			P:        &p.Image,
			V:        p.Image,
			Usage:    "Specify k3s image used for the node(s)",
			Required: false,
		},
	}
	return fs
}

func (p *K3d) GetDeleteFlags() []types.Flag {
	return []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
	}
}

func (p *K3d) GetJoinFlags() []types.Flag {
	return p.sharedFlags()
}

func (p *K3d) GetSSHFlags() []types.Flag {
	return []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
	}
}

func (p *K3d) MergeClusterOptions() error {
	opt, err := p.MergeConfig()
	if err != nil {
		return err
	}
	stateOption, err := p.GetProviderOptions(opt)
	if err != nil {
		return err
	}
	option := stateOption.(*k3d.Options)

	// merge options.
	source := reflect.ValueOf(&p.Options).Elem()
	target := reflect.ValueOf(option).Elem()
	utils.MergeConfig(source, target)

	return nil
}

func (p *K3d) GetCredentialFlags() []types.Flag {
	return []types.Flag{}
}

func (p *K3d) BindCredential() error {
	return nil
}

func (p *K3d) sharedFlags() []types.Flag {
	return []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
		{
			Name:  "master",
			P:     &p.Master,
			V:     p.Master,
			Usage: "Number of master node",
		},
		{
			Name:  "worker",
			P:     &p.Worker,
			V:     p.Worker,
			Usage: "Number of worker node",
		},
		{
			Name:  "master-extra-args",
			P:     &p.MasterExtraArgs,
			V:     p.MasterExtraArgs,
			Usage: "Master extra arguments for k3s installer, wrapped in quotes. e.g.(--master-extra-args '--no-deploy metrics-server'), for more information, please see: https://rancher.com/docs/k3s/latest/en/installation/install-options/server-config/",
		},
		{
			Name:  "worker-extra-args",
			P:     &p.WorkerExtraArgs,
			V:     p.WorkerExtraArgs,
			Usage: "Worker extra arguments for k3s installer, wrapped in quotes. e.g.(--worker-extra-args '--node-taint key=value:NoExecute'), for more information, please see: https://rancher.com/docs/k3s/latest/en/installation/install-options/agent-config/",
		},
		{
			Name:  "registry",
			P:     &p.Registry,
			V:     p.Registry,
			Usage: "K3s registry file, see: https://rancher.com/docs/k3s/latest/en/installation/private-registry",
		},
		{
			Name:     "masters-memory",
			P:        &p.MastersMemory,
			V:        p.MastersMemory,
			Usage:    "Memory limit imposed on the server nodes [From docker]",
			Required: false,
		},
		{
			Name:     "workers-memory",
			P:        &p.WorkersMemory,
			V:        p.WorkersMemory,
			Usage:    "Memory limit imposed on the agents nodes [From docker]",
			Required: false,
		},
	}
}
