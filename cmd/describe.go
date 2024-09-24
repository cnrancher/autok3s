package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	describeCmd = &cobra.Command{
		Use:     "describe",
		Short:   "Show details of a specific resource",
		Example: `  autok3s describe -n <cluster-name> -p <provider>`,
	}
	desProvider = ""
	name        = ""
)

func init() {
	describeCmd.Flags().StringVarP(&desProvider, "provider", "p", desProvider, "Provider is a module which provides an interface for managing cloud resources")
	describeCmd.Flags().StringVarP(&name, "name", "n", name, "cluster name")
}

// DescribeCommand returns the specified cluster details.
func DescribeCommand() *cobra.Command {
	describeCmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if name == "" {
			logrus.Fatalln("`-n` or `--name` must set to specify a cluster, i.e. autok3s describe -n <cluster-name>")
		}
		return nil
	}
	describeCmd.Run = func(_ *cobra.Command, _ []string) {
		describeCluster()
	}
	return describeCmd
}

func describeCluster() {
	allErr := make([]string, 0)
	kubeCfg := filepath.Join(common.CfgPath, common.KubeCfgFile)
	out := new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 0, '\t', 0)

	result, err := common.DefaultDB.FindCluster(name, desProvider)
	if err != nil {
		logrus.Fatalf("find cluster error %v", err)
	}
	for _, state := range result {
		// TODO skip harvester for historical data, will remove here after harvester provider added back
		if state.Provider == "harvester" {
			continue
		}
		provider, err := providers.GetProvider(state.Provider)
		if err != nil {
			logrus.Errorf("failed to get provider %v: %v", state.Provider, err)
			continue
		}
		provider.SetMetadata(&state.Metadata)
		_ = provider.SetOptions(state.Options)
		isExist, _, err := provider.IsClusterExist()
		if err != nil {
			logrus.Errorf("failed to check cluster %s exist, got error: %v ", state.Name, err)
			continue
		}
		if !isExist {
			allErr = append(allErr, fmt.Sprintf("cluster %s is not exist", name))
			continue
		}
		info := provider.DescribeCluster(kubeCfg)
		_, _ = fmt.Fprintf(out, "Name: %s\n", name)
		_, _ = fmt.Fprintf(out, "Provider: %s\n", info.Provider)
		_, _ = fmt.Fprintf(out, "Region: %s\n", info.Region)
		_, _ = fmt.Fprintf(out, "Zone: %s\n", info.Zone)
		_, _ = fmt.Fprintf(out, "Master: %s\n", info.Master)
		_, _ = fmt.Fprintf(out, "Worker: %s\n", info.Worker)
		_, _ = fmt.Fprintf(out, "IsHAMode: %v\n", info.IsHAMode)
		if info.IsHAMode {
			_, _ = fmt.Fprintf(out, "DataStoreType: %s\n", info.DataStoreType)
		}
		_, _ = fmt.Fprintf(out, "Status: %s\n", info.Status)
		_, _ = fmt.Fprintf(out, "Version: %s\n", info.Version)
		_, _ = fmt.Fprintf(out, "Nodes:%s\n", "")
		for _, node := range info.Nodes {
			_, _ = fmt.Fprintf(out, "  - internal-ip: %s\n", node.InternalIP)
			_, _ = fmt.Fprintf(out, "    external-ip: %s\n", node.ExternalIP)
			_, _ = fmt.Fprintf(out, "    instance-status: %s\n", node.InstanceStatus)
			_, _ = fmt.Fprintf(out, "    instance-id: %s\n", node.InstanceID)
			_, _ = fmt.Fprintf(out, "    roles: %s\n", node.Roles)
			_, _ = fmt.Fprintf(out, "    status: %s\n", node.Status)
			_, _ = fmt.Fprintf(out, "    hostname: %s\n", node.HostName)
			_, _ = fmt.Fprintf(out, "    container-runtime: %s\n", node.ContainerRuntimeVersion)
			_, _ = fmt.Fprintf(out, "    version: %s\n", node.Version)
		}
	}
	for _, e := range allErr {
		_, _ = fmt.Fprintf(out, "%s\n", e)
	}
	_ = out.Flush()
}
