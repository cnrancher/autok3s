package cmd

import (
	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	sshCmd = &cobra.Command{
		Use:   "ssh",
		Short: "Connect to a K3s node through SSH",
	}

	sProvider = ""
	sp        providers.Provider
)

func init() {
	sshCmd.Flags().StringVarP(&sProvider, "provider", "p", sProvider, "Provider is a module which provides an interface for managing cloud resources")
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

		sshCmd.Flags().AddFlagSet(utils.ConvertFlags(sshCmd, sp.GetCredentialFlags()))
		sshCmd.Flags().AddFlagSet(utils.ConvertFlags(sshCmd, sp.GetSSHFlags()))
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
		node := ""
		if len(args) > 0 {
			node = args[0]
		}
		if err := sp.SSHK3sNode(node); err != nil {
			logrus.Fatalln(err)
		}
	}

	return sshCmd
}
