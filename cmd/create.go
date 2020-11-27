package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create k3s cluster",
		Example: `  autok3s create \
    --provider alibaba \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret> \
    --master 1`,
	}

	cProvider = ""
	cp        providers.Provider

	cSSH = &types.SSH{
		User: "root",
		Port: "22",
	}
)

func init() {
	createCmd.Flags().StringVarP(&cProvider, "provider", "p", cProvider, "Provider is a module which provides an interface for managing cloud resources")
	createCmd.Flags().StringVar(&cSSH.User, "ssh-user", cSSH.User, "SSH user for host")
	createCmd.Flags().StringVar(&cSSH.Port, "ssh-port", cSSH.Port, "SSH port for host")
	createCmd.Flags().StringVar(&cSSH.SSHKeyPath, "ssh-key-path", cSSH.SSHKeyPath, "SSH private key path")
	createCmd.Flags().StringVar(&cSSH.SSHKeyPassphrase, "ssh-key-pass", cSSH.SSHKeyPassphrase, "SSH passphrase of private key")
	createCmd.Flags().StringVar(&cSSH.SSHCertPath, "ssh-key-cert-path", cSSH.SSHCertPath, "SSH private key certificate path")
	createCmd.Flags().StringVar(&cSSH.Password, "ssh-password", cSSH.Password, "SSH login password")
	createCmd.Flags().BoolVar(&cSSH.SSHAgentAuth, "ssh-agent", cSSH.SSHAgentAuth, "Enable ssh agent")
}

func CreateCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			cp = reg
		}

		createCmd.Flags().AddFlagSet(cp.GetCredentialFlags(createCmd))
		createCmd.Flags().AddFlagSet(cp.GetCreateFlags(createCmd))
	}

	createCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}
		common.InitPFlags(cmd, cp)
		return common.MakeSureCredentialFlag(cmd.Flags(), cp)
	}

	createCmd.Run = func(cmd *cobra.Command, args []string) {
		// generate cluster name. e.g. input: "--name k3s1 --region cn-hangzhou" output: "k3s1.cn-hangzhou"
		cp.GenerateClusterName()

		// create k3s cluster with generated cluster name.
		if err := cp.CreateK3sCluster(cSSH); err != nil {
			logrus.Errorln(err)
			if rErr := cp.Rollback(); rErr != nil {
				logrus.Fatalln(rErr)
			}
		}
	}

	return createCmd
}
