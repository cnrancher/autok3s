package cmd

import (
	"github.com/Jason-ZW/autok3s/pkg/providers"
	"github.com/Jason-ZW/autok3s/pkg/utils"

	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create k3s cluster",
		Example: `  autok3s create --provider alibaba`,
	}

	provider = ""
)

func init() {
	createCmd.Flags().StringVarP(&provider, "provider", "p", provider, "provider is a module which provides an interface for managing cloud resources")
}

func CreateCommand() *cobra.Command {
	pStr := utils.FlagHackLookup("--provider")
	if pStr != "" {
		p := providers.Register(pStr)
		createCmd.Flags().AddFlagSet(p.GetCreateFlags())
	}

	createCmd.Run = func(cmd *cobra.Command, args []string) {
	}

	return createCmd
}
