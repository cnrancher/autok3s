package cmd

import (
	"os"

	"github.com/Jason-ZW/autok3s/pkg/cluster"
	"github.com/Jason-ZW/autok3s/pkg/common"
	"github.com/Jason-ZW/autok3s/pkg/providers"
	"github.com/Jason-ZW/autok3s/pkg/utils"

	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "get",
		Short:     "Display one or many resources",
		ValidArgs: []string{"provider", "cluster"},
		Args:      cobra.RangeArgs(1, 2),
		Example:   `  autok3s get provider`,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "provider":
			getProvider(args)
		case "cluster":
			getCluster(args)
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

func getCluster(args []string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader([]string{"Name", "Provider", "Masters", "Workers"})

	input := ""
	if len(args) > 1 {
		input = args[1]
	}

	v := common.CfgPath
	if v == "" {
		logrus.Fatalln("state path is empty\n")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		logrus.Fatalf("read state file error, msg: %s\n", err.Error())
	}

	result, err := cluster.ConvertToClusters(clusters)
	if err != nil {
		logrus.Fatalf("failed to unmarshal state file, msg: %s\n", err.Error())
	}

	for _, r := range result {
		if input != "" {
			if input == r.Name {
				table.Append([]string{
					r.Name,
					r.Provider,
					r.Master,
					r.Worker,
				})
			}
		} else {
			table.Append([]string{
				r.Name,
				r.Provider,
				r.Master,
				r.Worker,
			})
		}
	}

	table.Render()
}
