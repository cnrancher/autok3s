package cmd

import (
	"os"

	"github.com/cnrancher/autok3s/pkg/cluster"

	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:     "list",
		Short:   "Display all K3s clusters",
		Example: `  autok3s list`,
	}
)

func ListCommand() *cobra.Command {
	listCmd.Run = func(cmd *cobra.Command, args []string) {
		listCluster()
	}
	return listCmd
}

func listCluster() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader([]string{"Name", "Region", "Provider", "Status", "Masters", "Workers", "Version"})

	filters, err := cluster.ListClusters()
	if err != nil {
		logrus.Fatalln(err)
	}

	for _, f := range filters {
		table.Append([]string{
			f.Name,
			f.Region,
			f.Provider,
			f.Status,
			f.Master,
			f.Worker,
			f.Version,
		})
	}

	table.Render()
}
