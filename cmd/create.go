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
	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a K3s cluster",
	}

	cProvider = ""
	cp        providers.Provider
)

func init() {
	createCmd.Flags().StringVarP(&cProvider, "provider", "p", cProvider, "Provider is a module which provides an interface for managing cloud resources")
}

// CreateCommand create command.
func CreateCommand() *cobra.Command {
	// load dynamic provider flags.
	pStr := common.FlagHackLookup("--provider")
	if pStr != "" {
		if reg, err := providers.GetProvider(pStr); err != nil {
			logrus.Fatalln(err)
		} else {
			cp = reg
		}

		createCmd.Flags().AddFlagSet(utils.ConvertFlags(createCmd, cp.GetCredentialFlags()))
		createCmd.Flags().AddFlagSet(utils.ConvertFlags(createCmd, cp.GetOptionFlags()))
		createCmd.Flags().AddFlagSet(utils.ConvertFlags(createCmd, cp.GetCreateFlags()))
		createCmd.Example = cp.GetUsageExample("create")
		createCmd.Use = fmt.Sprintf("create -p %s", pStr)
	}

	createCmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if cProvider == "" {
			logrus.Fatalln("required flag(s) \"--provider\" not set")
		}
		common.BindEnvFlags(cmd)
		if err := common.MakeSureCredentialFlag(cmd.Flags(), cp); err != nil {
			return err
		}
		utils.ValidateRequiredFlags(cmd.Flags())
		return nil
	}

	createCmd.Run = func(_ *cobra.Command, _ []string) {
		// generate cluster name. i.e. input: "--name k3s1 --region cn-hangzhou" output: "k3s1.cn-hangzhou.<provider>".
		cp.GenerateClusterName()
		if err := cp.BindCredential(); err != nil {
			logrus.Fatalln(err)
		}
		if err := cp.CreateCheck(); err != nil {
			logrus.Fatalln(err)
		}

		// create k3s cluster with generated cluster name.
		if err := cp.CreateK3sCluster(); err != nil {
			logrus.Fatalln(err)
		}
	}

	return createCmd
}
