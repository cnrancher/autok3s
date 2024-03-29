package addon

import "github.com/spf13/cobra"

var (
	addonCmd = &cobra.Command{
		Use:   "add-ons",
		Short: "The add-ons management",
		Long:  "The add-ons command helps to manage add-ons which can install to multiple K3s clusters",
	}
)

func Command() *cobra.Command {
	addonCmd.AddCommand(CreateCmd(), UpdateCmd(), RemoveCmd(), ListCmd(), GetCmd())
	return addonCmd
}
