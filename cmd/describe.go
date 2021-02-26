package cmd

import (
	"fmt"
	"os"
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

func DescribeCommand() *cobra.Command {
	describeCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if name == "" {
			logrus.Fatalln("`-n` or `--name` must set to specify a cluster, i.e. autok3s describe -n <cluster-name>")
		}
		return nil
	}
	describeCmd.Run = func(cmd *cobra.Command, args []string) {
		describeCluster()
	}
	return describeCmd
}

func describeCluster() {
	allErr := make([]string, 0)
	kubeCfg := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	out := new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 0, '\t', 0)

	result, err := common.DefaultDB.FindCluster(name, desProvider)
	if err != nil {
		logrus.Fatalf("find cluster error %v", err)
	}
	for _, state := range result {
		provider, err := providers.GetProvider(state.Provider)
		if err != nil {
			logrus.Errorf("failed to get provider %v: %v", state.Provider, err)
			continue
		}
		provider.SetMetadata(&state.Metadata)
		provider.SetOptions(state.Options)
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
		fmt.Fprintf(out, "Name: %s\n", name)
		fmt.Fprintf(out, "Provider: %s\n", info.Provider)
		fmt.Fprintf(out, "Region: %s\n", info.Region)
		fmt.Fprintf(out, "Zone: %s\n", info.Zone)
		fmt.Fprintf(out, "Master: %s\n", info.Master)
		fmt.Fprintf(out, "Worker: %s\n", info.Worker)
		fmt.Fprintf(out, "Status: %s\n", info.Status)
		fmt.Fprintf(out, "Version: %s\n", info.Version)
		fmt.Fprintf(out, "Nodes:%s\n", "")
		for _, node := range info.Nodes {
			fmt.Fprintf(out, "  - internal-ip: %s\n", node.InternalIP)
			fmt.Fprintf(out, "    external-ip: %s\n", node.ExternalIP)
			fmt.Fprintf(out, "    instance-status: %s\n", node.InstanceStatus)
			fmt.Fprintf(out, "    instance-id: %s\n", node.InstanceID)
			fmt.Fprintf(out, "    roles: %s\n", node.Roles)
			fmt.Fprintf(out, "    status: %s\n", node.Status)
			fmt.Fprintf(out, "    hostname: %s\n", node.HostName)
			fmt.Fprintf(out, "    container-runtime: %s\n", node.ContainerRuntimeVersion)
			fmt.Fprintf(out, "    version: %s\n", node.Version)
		}
	}
	for _, e := range allErr {
		fmt.Fprintf(out, "%s\n", e)
	}
	out.Flush()
}
