package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Jason-ZW/autok3s/pkg/cluster"
	"github.com/Jason-ZW/autok3s/pkg/common"
	"github.com/Jason-ZW/autok3s/pkg/utils"

	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	getCmd = &cobra.Command{
		Use:       "get",
		Short:     "Display one or many resources",
		ValidArgs: []string{"cluster"},
		Args:      cobra.ExactArgs(1),
		Example:   `  autok3s get cluster`,
	}

	name, region string
)

func init() {
	getCmd.Flags().StringVar(&name, "name", name, "Cluster name")
	getCmd.Flags().StringVar(&region, "region", region, "Physical locations (data centers) that spread all over the world to reduce the network latency")
}

func GetCommand() *cobra.Command {
	getCmd.Run = func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "cluster":
			getCluster()
		}
	}
	return getCmd
}

func getCluster() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("")
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader([]string{"Name", "Region", "Provider", "Masters", "Workers"})

	v := common.CfgPath
	if v == "" {
		logrus.Fatalln("state path is empty")
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
		if name != "" && region != "" {
			if fmt.Sprintf("%s.%s", name, region) == r.Name {
				table.Append([]string{
					name,
					region,
					r.Provider,
					r.Master,
					r.Worker,
				})
			}
		} else if name != "" {
			if strings.Contains(r.Name, name) {
				table.Append([]string{
					r.Name[:strings.LastIndex(r.Name, ".")],
					r.Name[strings.LastIndex(r.Name, ".")+1:],
					r.Provider,
					r.Master,
					r.Worker,
				})
			}
		} else if region != "" {
			if strings.Contains(r.Name, region) {
				table.Append([]string{
					r.Name[:strings.LastIndex(r.Name, ".")],
					r.Name[strings.LastIndex(r.Name, ".")+1:],
					r.Provider,
					r.Master,
					r.Worker,
				})
			}
		} else {
			table.Append([]string{
				r.Name[:strings.LastIndex(r.Name, ".")],
				r.Name[strings.LastIndex(r.Name, ".")+1:],
				r.Provider,
				r.Master,
				r.Worker,
			})
		}
	}

	table.Render()
}
