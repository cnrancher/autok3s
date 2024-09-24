package cmd

import (
	"fmt"

	"github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	joinCmd = &cobra.Command{
		Use:   "join",
		Short: "Join one or more K3s node(s) to an existing cluster",
	}

	jProvider = ""
	jp        providers.Provider
)

func init() {
	joinCmd.Flags().StringVarP(&jProvider, "provider", "p", jProvider, "Provider is a module which provides an interface for managing cloud resources")
}

// JoinCommand join command.
func JoinCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			jp = reg
		}

		joinCmd.Flags().AddFlagSet(utils.ConvertFlags(joinCmd, jp.GetCredentialFlags()))
		joinCmd.Flags().AddFlagSet(utils.ConvertFlags(joinCmd, jp.GetJoinFlags()))
		joinCmd.Example = jp.GetUsageExample("join")
		joinCmd.Use = fmt.Sprintf("join -p %s", pStr)
	}

	joinCmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if jProvider == "" {
			logrus.Fatalln("required flag(s) \"[provider]\" not set")
		}
		common.BindEnvFlags(cmd)
		err := jp.MergeClusterOptions()
		if err != nil {
			return err
		}

		if err = common.MakeSureCredentialFlag(cmd.Flags(), jp); err != nil {
			return err
		}
		utils.ValidateRequiredFlags(cmd.Flags())
		return nil
	}

	joinCmd.Run = func(_ *cobra.Command, _ []string) {
		// generate cluster name. i.e. input: "--name k3s1 --region cn-hangzhou" output: "k3s1.cn-hangzhou".
		jp.GenerateClusterName()
		if err := jp.JoinCheck(); err != nil {
			logrus.Fatalln(err)
		}
		// join k3s node to the cluster which named with generated cluster name.
		if err := jp.JoinK3sNode(); err != nil {
			logrus.Fatalln(err)
		}
	}

	return joinCmd
}
