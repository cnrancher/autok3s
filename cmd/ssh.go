package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:   "ssh",
		Short: "SSH k3s node",
	}

	sProvider = ""
	sp        providers.Provider

	sSSH = &types.SSH{}
)

func init() {
	sshCmd.Flags().StringVarP(&sProvider, "provider", "p", sProvider, "Provider is a module which provides an interface for managing cloud resources")
	sshCmd.Flags().StringVar(&sSSH.User, "ssh-user", sSSH.User, "SSH user for host")
	sshCmd.Flags().StringVar(&sSSH.Port, "ssh-port", sSSH.Port, "SSH port for host")
	sshCmd.Flags().StringVar(&sSSH.SSHKeyPath, "ssh-key-path", sSSH.SSHKeyPath, "SSH private key path")
	sshCmd.Flags().StringVar(&sSSH.SSHKeyPassphrase, "ssh-key-pass", sSSH.SSHKeyPassphrase, "SSH passphrase of private key")
	sshCmd.Flags().StringVar(&sSSH.SSHCertPath, "ssh-key-cert-path", sSSH.SSHCertPath, "SSH private key certificate path")
	sshCmd.Flags().StringVar(&sSSH.Password, "ssh-password", sSSH.Password, "SSH login password")
	sshCmd.Flags().BoolVar(&sSSH.SSHAgentAuth, "ssh-agent", sSSH.SSHAgentAuth, "Enable ssh agent")
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
		sshCmd.Example = sp.GetUsageExample("ssh")
	}

	sshCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if sProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}
		common.InitPFlags(cmd, sp)
		err := sp.MergeClusterOptions()
		if err != nil {
			return err
		}
		return common.MakeSureCredentialFlag(cmd.Flags(), sp)
	}

	sshCmd.Run = func(cmd *cobra.Command, args []string) {
		sp.GenerateClusterName()

		if err := sp.SSHK3sNode(sSSH); err != nil {
			logrus.Fatalln(err)
		}
	}

	return sshCmd
}
