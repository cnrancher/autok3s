package airgap

import (
	"github.com/spf13/cobra"
)

var (
	airgap = &cobra.Command{
		Use:   "airgap",
		Short: "The airgap packages management.",
		Long:  "The airgap command manages the airgap package for k3s.",
	}
)

func Command() *cobra.Command {
	airgap.AddCommand(
		listCmd,
		createCmd,
		removeCmd,
		updateCmd,
		importCmd,
		exportCmd,
		updateScriptCmd,
	)
	return airgap
}
