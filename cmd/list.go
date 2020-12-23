package cmd

import (
	"fmt"
	"os"

	c "github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

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
	table.SetHeader([]string{"Name", "Region", "Provider", "Status", "Masters", "Workers", "Version"})

	v := common.CfgPath
	if v == "" {
		logrus.Fatalln("state path is empty")
	}
	// get all clusters from state
	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		logrus.Fatalf("read state file error, msg: %v\n", err)
	}

	result, err := cluster.ConvertToClusters(clusters)
	if err != nil {
		logrus.Fatalf("failed to unmarshal state file, msg: %v\n", err)
	}

	var (
		p           providers.Provider
		filters     []*types.ClusterInfo
		clusterList []*types.Cluster
	)

	kubeCfg := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	for _, r := range result {
		p, err = c.GetProviderByState(r)
		if err != nil {
			logrus.Errorf("failed to convert cluster options for cluster %s", r.Name)
			continue
		}
		isExist, _, err := p.IsClusterExist()
		if err != nil {
			logrus.Errorf("failed to check cluster %s exist, got error: %v ", r.Name, err)
			continue
		}
		if !isExist {
			logrus.Warnf("cluster %s is not exist, will remove from config", r.Name)
			// remove kube config if cluster not exist
			if err := cluster.OverwriteCfg(r.Name); err != nil {
				logrus.Errorf("failed to remove unexist cluster %s from kube config", r.Name)
			}
			continue
		}

		filters = append(filters, p.GetCluster(kubeCfg))
		clusterList = append(clusterList, &types.Cluster{
			Metadata: r.Metadata,
			Options:  r.Options,
			Status:   r.Status,
		})
	}

	// remove useless clusters from .state.
	if err := cluster.FilterState(clusterList); err != nil {
		logrus.Errorf("failed to remove useless clusters\n")
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
