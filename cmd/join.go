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
	joinCmd = &cobra.Command{
		Use:   "join",
		Short: "Join k3s node",
		Example: `  autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --token <k3s token> \
    --ip <k3s master/lb ip> \
    --access-key <access-key> \
    --access-secret <access-secret> \
    --worker 1`,
	}

	jProvider = ""
	jp        providers.Provider

	jSSH = &types.SSH{
		SSHKeyPath: "~/.ssh/id_rsa",
		User:       "root",
		Port:       "22",
	}
)

func init() {
	joinCmd.Flags().StringVarP(&jProvider, "provider", "p", jProvider, "Provider is a module which provides an interface for managing cloud resources")
	joinCmd.Flags().StringVar(&jSSH.User, "ssh-user", jSSH.User, "SSH user for host")
	joinCmd.Flags().StringVar(&jSSH.Port, "ssh-port", jSSH.Port, "SSH port for host")
	joinCmd.Flags().StringVar(&jSSH.SSHKeyPath, "ssh-key-path", jSSH.SSHKeyPath, "SSH private key path")
	joinCmd.Flags().StringVar(&jSSH.SSHKeyPassphrase, "ssh-key-pass", jSSH.SSHKeyPassphrase, "SSH passphrase of private key")
	joinCmd.Flags().StringVar(&jSSH.SSHCertPath, "ssh-key-cert-path", jSSH.SSHCertPath, "SSH private key certificate path")
	joinCmd.Flags().StringVar(&jSSH.Password, "ssh-password", jSSH.Password, "SSH login password")
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

		joinCmd.Flags().AddFlagSet(jp.GetCredentialFlags(joinCmd))
		joinCmd.Flags().AddFlagSet(jp.GetJoinFlags(joinCmd))
	}

	joinCmd.Run = func(cmd *cobra.Command, args []string) {
		if jProvider == "" {
			logrus.Fatalln("required flags(s) \"[provider]\" not set")
		}

		// must bind after dynamic provider flags loaded.
		common.BindPFlags(cmd, jp)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

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
