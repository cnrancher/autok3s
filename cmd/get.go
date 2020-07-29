package cmd

import (
	"os"

	"github.com/Jason-ZW/autok3s/pkg/providers"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "get",
		Short:     "Display one or many resources",
		ValidArgs: []string{"provider"},
		Args:      cobra.RangeArgs(1, 2),
		Example:   `  autok3s get provider`,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "provider":
			getProvider(args)
		}
	}
	return cmd
}

func getProvider(args []string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader([]string{"Provider", "InTree"})

	input := ""
	if len(args) > 1 {
		input = args[1]
	}

	for _, v := range providers.SupportedProviders(input) {
		table.Append(v)
	}
	table.Render()
}
