package native

import (
	"strings"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/native"

	"github.com/rancher/wrangler/pkg/slice"
)

const createUsageExample = `  autok3s -d create \
    --provider native \
    --name <cluster name> \
    --ssh-user <ssh-user> \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master-ips> \
    --worker-ips <worker-ips>
`

const joinUsageExample = `  autok3s -d join \
    --provider native \
    --name <cluster name> \
    --ssh-user <ssh-user> \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master-ips> \
    --worker-ips <worker-ips>
`

const deleteUsageExample = `  autok3s -d delete \
    --provider native \
    --name <cluster name>
`

const sshUsageExample = `  autok3s ssh \
    --provider native \
    --name <cluster name>
`

// GetUsageExample returns native usage example prompt.
func (p *Native) GetUsageExample(action string) string {
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
		return "not support"
	}
}

// GetCreateFlags returns native create flags.
func (p *Native) GetCreateFlags() []types.Flag {
	cSSH := p.GetSSHConfig()
	p.SSH = *cSSH
	fs := p.GetClusterOptions()
	fs = append(fs, p.GetCreateOptions()...)
	return fs
}

// GetOptionFlags returns native option flags.
func (p *Native) GetOptionFlags() []types.Flag {
	return p.sharedFlags()
}

// GetJoinFlags returns native join flags.
func (p *Native) GetJoinFlags() []types.Flag {
	fs := p.sharedFlags()
	fs = append(fs, p.GetClusterOptions()...)
	fs = append(fs, types.Flag{
		Name:  "ip",
		P:     &p.IP,
		V:     p.IP,
		Usage: "IP for an existing k3s server",
	})
	return fs
}

// GetSSHFlags returns native ssh flags.
func (p *Native) GetSSHFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:      "name",
			P:         &p.Name,
			V:         p.Name,
			Usage:     "Set the name of the kubeconfig context",
			ShortHand: "n",
			Required:  true,
		},
	}
	fs = append(fs, p.GetSSHOptions()...)

	return fs
}

// GetDeleteFlags return native delete flags.
func (p *Native) GetDeleteFlags() []types.Flag {
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

// GetCredentialFlags return native credential flags.
func (p *Native) GetCredentialFlags() []types.Flag {
	return []types.Flag{}
}

// GetSSHConfig return native ssh config.
func (p *Native) GetSSHConfig() *types.SSH {
	ssh := &types.SSH{
		SSHUser: defaultUser,
		SSHPort: "22",
	}
	return ssh
}

// BindCredential bind native credential.
func (p *Native) BindCredential() error {
	return nil
}

// MergeClusterOptions merge native cluster options.
func (p *Native) MergeClusterOptions() error {
	opt, err := p.MergeConfig()
	if err != nil {
		return err
	}
	if opt != nil {
		stateOption, err := p.GetProviderOptions(opt)
		if err != nil {
			return err
		}
		option := stateOption.(*native.Options)
		// merge options.
		if p.MasterIps != "" {
			mergedMasterIps := []string{}
			masterIps := []string{}
			if option.MasterIps != "" {
				masterIps = strings.Split(option.MasterIps, ",")
			}
			optionMasterIps := strings.Split(p.MasterIps, ",")
			for _, ip := range optionMasterIps {
				if !slice.ContainsString(masterIps, ip) {
					mergedMasterIps = append(mergedMasterIps, ip)
				}
			}
			p.MasterIps = strings.Join(append(masterIps, mergedMasterIps...), ",")
		}

		if p.WorkerIps != "" {
			mergedWorkerIps := []string{}
			workerIps := []string{}
			if option.WorkerIps != "" {
				workerIps = strings.Split(option.WorkerIps, ",")
			}
			optionWorkerIps := strings.Split(p.WorkerIps, ",")
			for _, ip := range optionWorkerIps {
				if !slice.ContainsString(workerIps, ip) {
					mergedWorkerIps = append(mergedWorkerIps, ip)
				}
			}

			p.WorkerIps = strings.Join(append(workerIps, mergedWorkerIps...), ",")
		}
	}

	return nil
}

func (p *Native) sharedFlags() []types.Flag {
	fs := []types.Flag{
		{
			Name:  "master-ips",
			P:     &p.MasterIps,
			V:     p.MasterIps,
			Usage: "Public IPs of master nodes on which to install agent, multiple IPs are separated by commas",
		},
		{
			Name:  "worker-ips",
			P:     &p.WorkerIps,
			V:     p.WorkerIps,
			Usage: "Public IPs of worker nodes on which to install agent, multiple IPs are separated by commas",
		},
	}

	return fs
}
