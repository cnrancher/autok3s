package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	joinCmd = &cobra.Command{
		Use:   "join",
		Short: "Join k3s node",
	}

	jProvider = ""
	jp        providers.Provider

	jSSH = &types.SSH{
		Port: "22",
	}
)

func init() {
	joinCmd.Flags().StringVarP(&jProvider, "provider", "p", jProvider, "Provider is a module which provides an interface for managing cloud resources")
}

func JoinCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			jp = reg
		}
		jSSH = jp.GetSSHConfig()
		joinCmd.Flags().StringVar(&jSSH.User, "ssh-user", jSSH.User, "SSH user for host")
		joinCmd.Flags().StringVar(&jSSH.Port, "ssh-port", jSSH.Port, "SSH port for host")
		joinCmd.Flags().StringVar(&jSSH.SSHKeyPath, "ssh-key-path", jSSH.SSHKeyPath, "SSH private key path")
		joinCmd.Flags().StringVar(&jSSH.SSHKeyPassphrase, "ssh-key-pass", jSSH.SSHKeyPassphrase, "SSH passphrase of private key")
		joinCmd.Flags().StringVar(&jSSH.SSHCertPath, "ssh-key-cert-path", jSSH.SSHCertPath, "SSH private key certificate path")
		joinCmd.Flags().StringVar(&jSSH.Password, "ssh-password", jSSH.Password, "SSH login password")
		joinCmd.Flags().BoolVar(&jSSH.SSHAgentAuth, "ssh-agent", jSSH.SSHAgentAuth, "Enable ssh agent")

		joinCmd.Flags().AddFlagSet(jp.GetCredentialFlags(joinCmd))
		joinCmd.Flags().AddFlagSet(jp.GetJoinFlags(joinCmd))
		joinCmd.Example = jp.GetUsageExample("join")
	}

	joinCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if jProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}
		common.InitPFlags(cmd, jp)
		err := jp.MergeClusterOptions()
		if err != nil {
			return err
		}

		return common.MakeSureCredentialFlag(cmd.Flags(), jp)
	}

	joinCmd.Run = func(cmd *cobra.Command, args []string) {
		// generate cluster name. e.g. input: "--name k3s1 --region cn-hangzhou" output: "k3s1.cn-hangzhou"
		jp.GenerateClusterName()

		// join k3s node to the cluster which named with generated cluster name.
		if err := jp.JoinK3sNode(jSSH); err != nil {
			logrus.Errorln(err)
			if rErr := jp.Rollback(); rErr != nil {
				logrus.Fatalln(rErr)
			}
		}
	}

	return joinCmd
}
