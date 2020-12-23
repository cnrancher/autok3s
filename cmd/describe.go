package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	c "github.com/cnrancher/autok3s/cmd/common"
	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	describeCmd = &cobra.Command{
		Use:     "describe",
		Short:   "Show details of a specific resource",
		Example: `  autok3s describe cluster <cluster name>`,
	}
)

func DescribeCommand() *cobra.Command {
	describeCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || len(args) < 2 {
			logrus.Fatalln("you must specify the type of resource to describe, e.g. autok3s describe cluster <cluster name>")
		}
		return nil
	}
	describeCmd.Run = func(cmd *cobra.Command, args []string) {
		describeCluster(args)
	}
	return describeCmd
}

func describeCluster(args []string) {
	if len(args) < 2 {
		logrus.Fatalln("you must specify the type of resource to describe, e.g. autok3s describe cluster <cluster name>")
	}
	resource := args[0]
	if resource != "cluster" {
		logrus.Fatalf("autok3s doesn't support resource type %s", resource)
	}
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

	resourceNames := args[1:]
	allErr := []string{}
	kubeCfg := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	out := new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 0, '\t', 0)

	for _, name := range resourceNames {
		notExist := true
		for _, r := range result {
			if r.Name == name {
				notExist = false
				p, err := c.GetProviderByState(r)
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
					allErr = append(allErr, fmt.Sprintf("cluster %s is not exist", name))
					continue
				}
				info := p.GetCluster(kubeCfg)
				fmt.Fprintf(out, "Name: %s\n", info.Name)
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
		}
		if notExist {
			allErr = append(allErr, fmt.Sprintf("cluster %s is not exist", name))
		}
	}
	for _, e := range allErr {
		fmt.Fprintf(out, "%s\n", e)
	}
	out.Flush()
}
