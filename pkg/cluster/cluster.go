package cluster

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Jason-ZW/autok3s/pkg/common"
	"github.com/Jason-ZW/autok3s/pkg/hosts"
	"github.com/Jason-ZW/autok3s/pkg/types"
	"github.com/Jason-ZW/autok3s/pkg/utils"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/config"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	masterCommand         = "curl -sLS http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn K3S_TOKEN='%s' INSTALL_K3S_EXEC='--tls-san %s' sh -\n"
	workerCommand         = "curl -sLS http://rancher-mirror.cnrancher.com/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn K3S_URL='https://%s:6443' K3S_TOKEN='%s' sh -\n"
	catCfgCommand         = "cat /etc/rancher/k3s/k3s.yaml"
	deployManifestCommand = "echo \"%s\" > \"%s/ui.yaml\""
)

func InitK3sCluster(cluster *types.Cluster) error {
	token, err := utils.RandomToken(16)
	if err != nil {
		return err
	}

	cluster.Token = token

	if len(cluster.MasterNodes) <= 0 || len(cluster.MasterNodes[0].InternalIPAddress) <= 0 {
		return errors.New("[cluster] master node internal ip address can not be empty\n")
	}

	url := cluster.MasterNodes[0].InternalIPAddress[0]
	publicIP := cluster.MasterNodes[0].PublicIPAddress[0]

	for _, master := range cluster.MasterNodes {
		if _, err := execute(&hosts.Host{Node: master}, fmt.Sprintf(masterCommand, cluster.Token, publicIP), true); err != nil {
			return err
		}
	}

	for _, worker := range cluster.WorkerNodes {
		if _, err := execute(&hosts.Host{Node: worker}, fmt.Sprintf(workerCommand, url, cluster.Token), true); err != nil {
			return err
		}
	}

	// get k3s cluster config.
	cfg, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, catCfgCommand, false)
	if err != nil {
		return err
	}

	if err := saveCfg(cfg, publicIP, cluster.Name); err != nil {
		return err
	}

	// write current cluster to state file.
	if err := saveState(cluster); err != nil {
		return err
	}

	// deploy additional manifests. e.g. UI(none/dashboard/octopus-ui).
	switch cluster.UI {
	case "dashboard":
		if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]},
			fmt.Sprintf(deployManifestCommand, fmt.Sprintf(dashboardTmpl, cluster.Repo), common.K3sManifestsDir), true); err != nil {
			return err
		}
	case "octopus-ui":
		if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]},
			fmt.Sprintf(deployManifestCommand, octopusTmpl, common.K3sManifestsDir), true); err != nil {
			return err
		}
	}

	return nil
}

func JoinK3sNode(merged, added *types.Cluster) error {
	if merged.Token == "" {
		return errors.New("[cluster] k3s token can not be empty\n")
	}

	if len(merged.MasterNodes) <= 0 || len(merged.MasterNodes[0].InternalIPAddress) <= 0 {
		return errors.New("[cluster] master node internal ip address can not be empty\n")
	}

	url := merged.MasterNodes[0].InternalIPAddress[0]

	// TODO: join master node will be added soon.
	for i := 0; i < len(added.WorkerNodes); i++ {
		for _, full := range merged.WorkerNodes {
			if added.WorkerNodes[i].InstanceID == full.InstanceID {
				if _, err := execute(&hosts.Host{Node: full}, fmt.Sprintf(workerCommand, url, merged.Token), true); err != nil {
					return err
				}
				break
			}
		}
	}

	// write current cluster to state file.
	return saveState(merged)
}

func ReadFromState(cluster *types.Cluster) ([]types.Cluster, error) {
	r := make([]types.Cluster, 0)
	v := common.CfgPath
	if v == "" {
		return r, errors.New("[cluster] cfg path is empty\n")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return r, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return r, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s\n", err.Error())
	}

	for _, c := range converts {
		if c.Provider == cluster.Provider && c.Name == cluster.Name {
			r = append(r, c)
		}
	}

	return r, nil
}

func AppendToState(cluster *types.Cluster) ([]types.Cluster, error) {
	v := common.CfgPath
	if v == "" {
		return nil, errors.New("[cluster] cfg path is empty\n")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return nil, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return nil, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s\n", err.Error())
	}

	index := -1

	for i, c := range converts {
		if c.Provider == cluster.Provider && c.Name == cluster.Name {
			index = i
			//r = append(r, *cluster)
		}
	}

	if index > -1 {
		converts[index] = *cluster
	} else {
		converts = append(converts, *cluster)
	}

	return converts, nil
}

func ConvertToClusters(origin []interface{}) ([]types.Cluster, error) {
	result := make([]types.Cluster, 0)

	b, err := yaml.Marshal(origin)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(b, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func execute(host *hosts.Host, cmd string, print bool) (string, error) {
	dialer, err := hosts.SSHDialer(host)
	if err != nil {
		return "", err
	}

	tunnel, err := dialer.OpenTunnel()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tunnel.Close()
	}()

	result, err := tunnel.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}

	if print {
		fmt.Printf("[dialer] execute command result: %s\n", result)
	}

	return result, nil
}

func saveState(cluster *types.Cluster) error {
	r, err := AppendToState(cluster)
	if err != nil {
		return err
	}

	v := common.CfgPath
	if v == "" {
		return errors.New("[cluster] cfg path is empty\n")
	}

	return utils.WriteYaml(r, v, common.StateFile)
}

func saveCfg(cfg, ip, context string) error {
	replacer := strings.NewReplacer(
		"127.0.0.1", ip,
		"localhost", ip,
		"default", context,
	)

	result := replacer.Replace(cfg)

	tempPath := fmt.Sprintf("%s/.kube", common.CfgPath)

	temp, err := ioutil.TempFile(tempPath, common.KubeCfgTempName)
	if err != nil {
		return fmt.Errorf("[cluster] generate kubecfg temp file error, msg=%s\n", err.Error())
	}
	defer func() {
		_ = temp.Close()
	}()

	err = utils.WriteBytesToYaml([]byte(result), tempPath, temp.Name()[strings.Index(temp.Name(), common.KubeCfgTempName):])
	if err != nil {
		return fmt.Errorf("[cluster] write content to kubecfg temp file error, msg=%s\n", err.Error())
	}

	return mergeCfg(context, temp.Name())
}

func mergeCfg(context, right string) error {
	defer func() {
		if err := os.Remove(right); err != nil {
			logrus.Errorf("[cluster] remove kubecfg temp file error, msg=%s\n", err)
		}
	}()

	if err := utils.EnsureFileExist(common.CfgPath, common.KubeCfgFile); err != nil {
		return fmt.Errorf("[cluster] ensure kubecfg exist error, msg=%s\n", err.Error())
	}

	if err := overwriteCfg(context); err != nil {
		return fmt.Errorf("[cluster] overwrite kubecfg error, msg=%s\n", err.Error())
	}

	if err := os.Setenv(clientcmd.RecommendedConfigPathEnvVar, fmt.Sprintf("%s:%s",
		fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile), right)); err != nil {
		return fmt.Errorf("[cluster] set env error when merging kubecfg, msg=%s\n", err.Error())
	}

	out := &bytes.Buffer{}
	opt := config.ViewOptions{
		Flatten:      true,
		PrintFlags:   genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme).WithDefaultOutput("yaml"),
		ConfigAccess: clientcmd.NewDefaultPathOptions(),
		IOStreams:    genericclioptions.IOStreams{Out: out},
	}
	_ = opt.Merge.Set("true")

	printer, err := opt.PrintFlags.ToPrinter()
	if err != nil {
		return fmt.Errorf("[cluster] generate view options error, msg=%s\n", err.Error())
	}
	opt.PrintObject = printer.PrintObj

	if err := opt.Run(); err != nil {
		return fmt.Errorf("[cluster] merging kubecfg error, msg=%s\n", err.Error())
	}

	return utils.WriteBytesToYaml(out.Bytes(), common.CfgPath, common.KubeCfgFile)
}

func overwriteCfg(context string) error {
	c, err := clientcmd.LoadFromFile(fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
	if err != nil {
		return err
	}

	if _, found := c.Clusters[context]; found {
		delete(c.Clusters, context)
	}

	if _, found := c.Contexts[context]; found {
		delete(c.Contexts, context)
	}

	if _, found := c.AuthInfos[context]; found {
		delete(c.AuthInfos, context)
	}

	return clientcmd.WriteToFile(*c, fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
}
