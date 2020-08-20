package cmd

import (
	"github.com/Jason-ZW/autok3s/cmd/common"
	"github.com/Jason-ZW/autok3s/pkg/providers"
	"github.com/Jason-ZW/autok3s/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create k3s cluster",
		Example: `  autok3s create --provider alibaba`,
	}

	cProvider = ""
	cp        providers.Provider

	cSSH = &types.SSH{
		SSHKey: "~/.ssh/id_rsa",
		User:   "root",
		Port:   "22",
	}
)

func init() {
	createCmd.Flags().StringVarP(&cProvider, "provider", "p", cProvider, "Provider is a module which provides an interface for managing cloud resources")
	createCmd.Flags().StringVar(&cSSH.User, "user", cSSH.User, "SSH user for host")
	createCmd.Flags().StringVar(&cSSH.Port, "ssh-port", cSSH.Port, "SSH port for host")
	createCmd.Flags().StringVar(&cSSH.SSHKey, "ssh-key", cSSH.SSHKey, "SSH private key path")
}

func CreateCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.Register(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			cp = reg
		}

		createCmd.Flags().AddFlagSet(cp.GetCredentialFlags(createCmd))
		createCmd.Flags().AddFlagSet(cp.GetCreateFlags(createCmd))
	}

	createCmd.Run = func(cmd *cobra.Command, args []string) {
		// must bind after dynamic provider flags loaded.
		common.BindPFlags(cmd, cp)

		// read options from config.
		if err := viper.ReadInConfig(); err != nil {
			logrus.Fatalln(err)
		}

		// sync config data to local cfg path.
		if err := viper.WriteConfig(); err != nil {
			logrus.Fatalln(err)
		}

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
