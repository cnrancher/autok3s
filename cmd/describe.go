package cmd

import (
	"fmt"
	"os"
	"strings"
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
	desProvider = ""
	region      = ""
)

func init() {
	describeCmd.Flags().StringVarP(&desProvider, "provider", "p", desProvider, "Provider is a module which provides an interface for managing cloud resources")
	describeCmd.Flags().StringVarP(&region, "region", "r", region, "the physical locations of your cluster instance")
}

func DescribeCommand() *cobra.Command {
	describeCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || len(args) < 2 {
			logrus.Fatalln("you must specify the type of resource to describe, i.e. autok3s describe cluster <cluster name>")
		}
		resource := args[0]
		if resource != "cluster" {
			logrus.Fatalf("autok3s doesn't support resource type %s", resource)
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
		logrus.Fatalln("you must specify the type of resource to describe, i.e. autok3s describe cluster <cluster name>")
	}
	v := common.CfgPath
	if v == "" {
		logrus.Fatalln("state path is empty")
	}
	// get all clusters from state
	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		logrus.Fatalf("read state file error, msg: %v", err)
	}

	result, err := cluster.ConvertToClusters(clusters)
	if err != nil {
		logrus.Fatalf("failed to unmarshal state file, msg: %v", err)
	}

	resourceNames := args[1:]
	allErr := make([]string, 0)
	kubeCfg := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	out := new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 0, '\t', 0)

	for _, name := range resourceNames {
		exist := false
		for _, r := range result {
			exist = isSpecifiedCluster(r.Name, name, region, desProvider)
			if exist {
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
				info := p.DescribeCluster(kubeCfg)
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
				break
			}
		}
		if !exist {
			allErr = append(allErr, fmt.Sprintf("cluster %s is not exist", name))
		}
	}
	for _, e := range allErr {
		fmt.Fprintf(out, "%s\n", e)
	}
	out.Flush()
}

func isSpecifiedCluster(context, name, region, provider string) bool {
	// context format is <name>.<region>.<provider>
	contextArray := strings.Split(context, ".")
	if region == "" && provider == "" {
		return contextArray[0] == name
	}
	if region != "" && provider != "" {
		return context == fmt.Sprintf("%s.%s.%s", name, region, provider)
	}
	if region != "" && len(contextArray) == 3 {
		return contextArray[0] == name && contextArray[1] == region
	}
	if provider != "" && len(contextArray) == 3 {
		return contextArray[0] == name && contextArray[2] == provider
	}
	return false
}
