package cmd

import (
	"os"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/providers/alibaba"
	"github.com/cnrancher/autok3s/pkg/types"
	typesAli "github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/ghodss/yaml"
	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:     "list",
		Short:   "List K3s clusters",
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
	table.SetHeader([]string{"Name", "Region", "Provider", "Masters", "Workers"})

	v := common.CfgPath
	if v == "" {
		logrus.Fatalln("state path is empty")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		logrus.Fatalf("read state file error, msg: %s\n", err)
	}

	result, err := cluster.ConvertToClusters(clusters)
	if err != nil {
		logrus.Fatalf("failed to unmarshal state file, msg: %s\n", err)
	}

	var (
		p         providers.Provider
		filters   []*types.Cluster
		removeCtx []string
	)

	// filter useless clusters & contexts.
	for _, r := range result {
		switch r.Provider {
		case "alibaba":
			region := r.Name[strings.LastIndex(r.Name, ".")+1:]

			b, err := yaml.Marshal(r.Options)
			if err != nil {
				logrus.Debugf("failed to convert cluster %s options\n", r.Name)
				removeCtx = append(removeCtx, r.Name)
				continue
			}

			option := &typesAli.Options{}
			if err := yaml.Unmarshal(b, option); err != nil {
				removeCtx = append(removeCtx, r.Name)
				logrus.Debugf("failed to convert cluster %s options\n", r.Name)
				continue
			}
			option.Region = region

			p = &alibaba.Alibaba{
				Metadata: r.Metadata,
				Options:  *option,
			}

			isExist, ids, err := p.IsClusterExist()
			if err != nil {
				logrus.Fatalln(err)
			}

			if isExist && len(ids) > 0 {
				filters = append(filters, &r)
			} else {
				removeCtx = append(removeCtx, r.Name)
			}
		}
	}

	// remove useless clusters from .state.
	if err := cluster.FilterState(filters); err != nil {
		logrus.Fatalf("failed to remove useless clusters\n")
	}

	// remove useless contexts from kubeCfg.
	for _, r := range removeCtx {
		if err := cluster.OverwriteCfg(r); err != nil {
			logrus.Fatalf("failed to remove useless contexts\n")
		}
	}

	for _, f := range filters {
		table.Append([]string{
			f.Name[:strings.LastIndex(f.Name, ".")],
			f.Name[strings.LastIndex(f.Name, ".")+1:],
			f.Provider,
			f.Master,
			f.Worker,
		})
	}

	table.Render()
}
