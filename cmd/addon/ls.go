package addon

import (
	"os"
	"strconv"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Aliases: []string{"ls"},
		Use:     "list",
		Short:   "List all add-on list.",
	}
)

func ListCmd() *cobra.Command {
	listCmd.Run = func(_ *cobra.Command, _ []string) {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetBorder(false)
		table.SetHeaderLine(false)
		table.SetColumnSeparator("")
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetHeader([]string{"Name", "Description", "Values"})

		addons, err := common.DefaultDB.ListAddon()
		if err != nil {
			logrus.Fatalln(err)
		}
		for _, addon := range addons {
			table.Append([]string{
				addon.Name,
				addon.Description,
				strconv.Itoa(len(addon.Values)),
			})
		}

		table.Render()
	}

	return listCmd
}
