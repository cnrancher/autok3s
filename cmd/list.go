package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

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
	jsonOut = false
)

func init() {
	listCmd.Flags().BoolVarP(&jsonOut, "json", "j", jsonOut, "json output")
}

// ListCommand returns clusters as list.
func ListCommand() *cobra.Command {
	listCmd.Run = func(_ *cobra.Command, _ []string) {
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
	table.SetHeader([]string{"Name", "Region", "Provider", "Status", "Masters", "Workers", "Version", "IsHAMode", "DataStoreType"})

	filters, err := cluster.ListClusters("")
	if err != nil {
		logrus.Fatalln(err)
	}

	if jsonOut {
		cl, _ := json.Marshal(filters)
		fmt.Println(string(cl))
		return
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
			strconv.FormatBool(f.IsHAMode),
			f.DataStoreType,
		})
	}

	table.Render()
}
