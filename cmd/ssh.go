package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	sshCmd = &cobra.Command{
		Use:   "ssh",
		Short: "SSH k3s node",
		Example: `  autok3s ssh \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh private key path> \
    --ssh-user root \
    --ssh-port 22 \
    --access-key <access-key> \
    --access-secret <access-secret>`,
	}

	sProvider = ""
	sp        providers.Provider

	sSSH = &types.SSH{
		SSHKeyPath: "~/.ssh/id_rsa",
		User:       "root",
		Port:       "22",
	}
)

func init() {
	sshCmd.Flags().StringVarP(&sProvider, "provider", "p", sProvider, "Provider is a module which provides an interface for managing cloud resources")
	sshCmd.Flags().StringVar(&sSSH.User, "ssh-user", sSSH.User, "SSH user for host")
	sshCmd.Flags().StringVar(&sSSH.Port, "ssh-port", sSSH.Port, "SSH port for host")
	sshCmd.Flags().StringVar(&sSSH.SSHKeyPath, "ssh-key-path", sSSH.SSHKeyPath, "SSH private key path")
	sshCmd.Flags().StringVar(&sSSH.SSHKeyPassphrase, "ssh-key-pass", sSSH.SSHKeyPassphrase, "SSH passphrase of private key")
	sshCmd.Flags().StringVar(&sSSH.SSHCertPath, "ssh-key-cert-path", sSSH.SSHCertPath, "SSH private key certificate path")
	sshCmd.Flags().StringVar(&sSSH.Password, "ssh-password", sSSH.Password, "SSH login password")
}

func SSHCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			sp = reg
		}

		sshCmd.Flags().AddFlagSet(sp.GetCredentialFlags(sshCmd))
		sshCmd.Flags().AddFlagSet(sp.GetSSHFlags(sshCmd))
	}

	sshCmd.Run = func(cmd *cobra.Command, args []string) {
		// must bind after dynamic provider flags loaded.
		common.BindPFlags(cmd, sp)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

		sp.GenerateClusterName()

		if err := sp.SSHK3sNode(); err != nil {
			logrus.Fatalln(err)
		}
	}

	return sshCmd
}
