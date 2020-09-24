package cluster

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/alibaba"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/config"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	initCommand       = "curl -sLS %s | %s INSTALL_K3S_REGISTRIES='%s' K3S_TOKEN='%s' INSTALL_K3S_EXEC='server %s --tls-san %s %s' INSTALL_K3S_VERSION='%s' sh -\n"
	joinCommand       = "curl -sLS %s | %s INSTALL_K3S_REGISTRIES='%s' K3S_URL='https://%s:6443' K3S_TOKEN='%s' INSTALL_K3S_EXEC='%s' INSTALL_K3S_VERSION='%s' sh -\n"
	catCfgCommand     = "cat /etc/rancher/k3s/k3s.yaml"
	dockerCommand     = "curl https://get.docker.com | VERSION=19.03 sh -s - %s\n"
	deployUICommand   = "echo \"%s\" | base64 -d > \"%s/ui.yaml\""
	getNodeCommand    = "kubectl get node --show-labels | grep %s | awk '{print $1\" \"$3\" \"$6}'"
	deleteNodeCommand = "kubectl delete node %s"
)

func InitK3sCluster(cluster *types.Cluster) error {
	logrus.Infof("[%s] executing init k3s cluster logic...\n", cluster.Provider)

	p, err := providers.GetProvider(cluster.Provider)
	if err != nil {
		return err
	}

	k3sScript := cluster.InstallScript
	k3sMirror := cluster.Mirror
	dockerMirror := cluster.DockerMirror

	if cluster.Registries != "" {
		if !strings.Contains(cluster.Registries, "https://registry-1.docker.io") {
			cluster.Registries += ",https://registry-1.docker.io"
		}
	}

	if cluster.Token == "" {
		token, err := utils.RandomToken(16)
		if err != nil {
			return err
		}
		cluster.Token = token
	}

	if len(cluster.MasterNodes) <= 0 || len(cluster.MasterNodes[0].InternalIPAddress) <= 0 {
		return errors.New("[cluster] master node internal ip address can not be empty")
	}

	publicIP := cluster.IP
	if cluster.IP == "" {
		cluster.IP = cluster.MasterNodes[0].InternalIPAddress[0]
		publicIP = cluster.MasterNodes[0].PublicIPAddress[0]
	}

	masterExtraArgs := cluster.MasterExtraArgs
	workerExtraArgs := cluster.WorkerExtraArgs

	if cluster.DataStore != "" {
		masterExtraArgs += " --datastore-endpoint " + cluster.DataStore
	}

	if cluster.Network != "" {
		masterExtraArgs += fmt.Sprintf(" --flannel-backend=%s", cluster.Network)
	}

	if cluster.ClusterCIDR != "" {
		masterExtraArgs += " --cluster-cidr " + cluster.ClusterCIDR
	}

	logrus.Infof("[%s] creating k3s master-%d...\n", cluster.Provider, 1)
	master0ExtraArgs := masterExtraArgs
	providerExtraArgs := p.GenerateMasterExtraArgs(cluster, cluster.MasterNodes[0])
	if providerExtraArgs != "" {
		master0ExtraArgs += providerExtraArgs
	}
	if err := initMaster(k3sScript, k3sMirror, dockerMirror, publicIP, master0ExtraArgs, cluster, cluster.MasterNodes[0]); err != nil {
		return err
	}
	logrus.Infof("[%s] successfully created k3s master-%d\n", cluster.Provider, 1)

	masterErrChan := make(chan error)
	masterWaitGroupDone := make(chan bool)
	masterWaitGroup := &sync.WaitGroup{}
	masterWaitGroup.Add(len(cluster.MasterNodes) - 1)

	for i, master := range cluster.MasterNodes {
		// skip first master nodes
		if i == 0 {
			continue
		}
		go func(i int, master types.Node) {
			logrus.Infof("[%s] creating k3s master-%d...\n", cluster.Provider, i+1)
			masterNExtraArgs := masterExtraArgs
			providerExtraArgs := p.GenerateMasterExtraArgs(cluster, master)
			if providerExtraArgs != "" {
				masterNExtraArgs += providerExtraArgs
			}
			initAdditionalMaster(masterWaitGroup, masterErrChan, k3sScript, k3sMirror, dockerMirror, publicIP, masterNExtraArgs, cluster, master)
			logrus.Infof("[%s] successfully created k3s master-%d\n", cluster.Provider, i+1)
		}(i, master)
	}

	go func() {
		masterWaitGroup.Wait()
		close(masterWaitGroupDone)
	}()

	select {
	case <-masterWaitGroupDone:
		break
	case err := <-masterErrChan:
		return err
	}

	workerErrChan := make(chan error)
	workerWaitGroupDone := make(chan bool)
	workerWaitGroup := &sync.WaitGroup{}
	workerWaitGroup.Add(len(cluster.WorkerNodes))

	for i, worker := range cluster.WorkerNodes {
		go func(i int, worker types.Node) {
			logrus.Infof("[%s] creating k3s worker-%d...\n", cluster.Provider, i+1)
			extraArgs := workerExtraArgs
			providerExtraArgs := p.GenerateWorkerExtraArgs(cluster, worker)
			if providerExtraArgs != "" {
				extraArgs += providerExtraArgs
			}
			initWorker(workerWaitGroup, workerErrChan, k3sScript, k3sMirror, dockerMirror, extraArgs, cluster, worker)
			logrus.Infof("[%s] successfully created k3s worker-%d\n", cluster.Provider, i+1)
		}(i, worker)
	}

	go func() {
		workerWaitGroup.Wait()
		close(workerWaitGroupDone)
	}()

	select {
	case <-workerWaitGroupDone:
		break
	case err := <-workerErrChan:
		return err
	}

	// get k3s cluster config.
	cfg, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, true, []string{catCfgCommand})
	if err != nil {
		return err
	}

	logrus.Infof("[%s] deploying additional manifests\n", cluster.Provider)

	// deploy additional UI manifests. e.g. (none/dashboard/octopus-ui).
	switch cluster.UI {
	case "dashboard":
		if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, false,
			[]string{fmt.Sprintf(deployUICommand, base64.StdEncoding.EncodeToString(
				[]byte(fmt.Sprintf(dashboardTmpl, cluster.Repo))), common.K3sManifestsDir)}); err != nil {
			return err
		}
	case "octopus-ui":
		if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, false,
			[]string{fmt.Sprintf(deployUICommand, base64.StdEncoding.EncodeToString(
				[]byte(octopusTmpl)), common.K3sManifestsDir)}); err != nil {
			return err
		}
	}

	logrus.Infof("[%s] successfully deployed additional manifests\n", cluster.Provider)

	// merge current cluster to kube config.
	if err := SaveCfg(cfg, publicIP, cluster.Name); err != nil {
		return err
	}

	// write current cluster to state file.
	if err := SaveState(cluster); err != nil {
		return err
	}

	logrus.Infof("[%s] successfully executed init k3s cluster logic\n", cluster.Provider)
	return nil
}

func JoinK3sNode(merged, added *types.Cluster) error {
	logrus.Infof("[%s] executing join k3s node logic\n", merged.Provider)

	p, err := providers.GetProvider(merged.Provider)
	if err != nil {
		return err
	}
	k3sScript := merged.InstallScript
	k3sMirror := merged.Mirror
	dockerMirror := merged.DockerMirror

	if merged.Registries != "" {
		if !strings.Contains(merged.Registries, "https://registry-1.docker.io") {
			merged.Registries += ",https://registry-1.docker.io"
		}
	}

	if merged.Token == "" {
		return errors.New("[cluster] k3s token can not be empty")
	}

	if merged.IP == "" {
		if len(merged.MasterNodes) <= 0 || len(merged.MasterNodes[0].InternalIPAddress) <= 0 {
			return errors.New("[cluster] master node internal ip address can not be empty")
		}
		merged.IP = merged.MasterNodes[0].InternalIPAddress[0]
	}

	errChan := make(chan error)
	waitGroupDone := make(chan bool)
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(len(added.MasterNodes) + len(added.WorkerNodes))

	for i := 0; i < len(added.Status.MasterNodes); i++ {
		for _, full := range merged.MasterNodes {
			extraArgs := merged.MasterExtraArgs
			if added.Status.MasterNodes[i].InstanceID == full.InstanceID {
				go func(i int, full types.Node) {
					logrus.Infof("[%s] joining k3s master-%d...\n", merged.Provider, i+1)
					additionalExtraArgs := p.GenerateMasterExtraArgs(added, full)
					if additionalExtraArgs != "" {
						extraArgs += additionalExtraArgs
					}
					joinMaster(waitGroup, errChan, k3sScript, k3sMirror, dockerMirror, extraArgs, merged, full)
					logrus.Infof("[%s] successfully joined k3s master-%d\n", merged.Provider, i+1)
				}(i, full)
				break
			}
		}
	}

	for i := 0; i < len(added.Status.WorkerNodes); i++ {
		for _, full := range merged.WorkerNodes {
			extraArgs := merged.WorkerExtraArgs
			if added.Status.WorkerNodes[i].InstanceID == full.InstanceID {
				go func(i int, full types.Node) {
					logrus.Infof("[%s] joining k3s worker-%d...\n", merged.Provider, i+1)
					additionalExtraArgs := p.GenerateWorkerExtraArgs(added, full)
					if additionalExtraArgs != "" {
						extraArgs += additionalExtraArgs
					}
					joinWorker(waitGroup, errChan, k3sScript, k3sMirror, dockerMirror, extraArgs, merged, full)
					logrus.Infof("[%s] successfully joined k3s worker-%d\n", merged.Provider, i+1)
				}(i, full)
				break
			}
		}
	}

	go func() {
		waitGroup.Wait()
		close(waitGroupDone)
	}()

	select {
	case <-waitGroupDone:
		break
	case err := <-errChan:
		return err
	}

	// sync master & worker numbers.
	merged.Master = strconv.Itoa(len(merged.MasterNodes))
	merged.Worker = strconv.Itoa(len(merged.WorkerNodes))

	// write current cluster to state file.
	if err := SaveState(merged); err != nil {
		return nil
	}

	logrus.Infof("[%s] successfully executed join k3s node logic\n", merged.Provider)
	return nil
}

func SSHK3sNode(ip string, cluster *types.Cluster) error {
	var node types.Node

	for _, n := range cluster.Status.MasterNodes {
		if n.PublicIPAddress[0] == ip {
			node = n
		}
	}

	for _, n := range cluster.Status.WorkerNodes {
		if n.PublicIPAddress[0] == ip {
			node = n
		}
	}

	return terminal(&hosts.Host{Node: node})
}

func ReadFromState(cluster *types.Cluster) ([]types.Cluster, error) {
	r := make([]types.Cluster, 0)
	v := common.CfgPath
	if v == "" {
		return r, errors.New("[cluster] cfg path is empty")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return r, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return r, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s", err)
	}

	for _, c := range converts {
		name := cluster.Name

		switch cluster.Provider {
		case "alibaba":
			if option, ok := cluster.Options.(alibaba.Options); ok {
				name = fmt.Sprintf("%s.%s", cluster.Name, option.Region)
			}
		}

		if c.Provider == cluster.Provider && c.Name == name {
			r = append(r, c)
		}
	}

	return r, nil
}

func AppendToState(cluster *types.Cluster) ([]types.Cluster, error) {
	v := common.CfgPath
	if v == "" {
		return nil, errors.New("[cluster] cfg path is empty")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return nil, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return nil, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s", err)
	}

	index := -1

	for i, c := range converts {
		if c.Provider == cluster.Provider && c.Name == cluster.Name {
			index = i
		}
	}

	if index > -1 {
		converts[index] = *cluster
	} else {
		converts = append(converts, *cluster)
	}

	return converts, nil
}

func DeleteState(name string, provider string) error {
	r, err := deleteClusterFromState(name, provider)
	if err != nil {
		return err
	}

	v := common.CfgPath
	if v == "" {
		return errors.New("[cluster] cfg path is empty")
	}

	return utils.WriteYaml(r, v, common.StateFile)
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

func SaveState(cluster *types.Cluster) error {
	r, err := AppendToState(cluster)
	if err != nil {
		return err
	}

	v := common.CfgPath
	if v == "" {
		return errors.New("[cluster] cfg path is empty")
	}

	return utils.WriteYaml(r, v, common.StateFile)
}

func FilterState(r []*types.Cluster) error {
	v := common.CfgPath
	if v == "" {
		return errors.New("[cluster] cfg path is empty")
	}

	return utils.WriteYaml(r, v, common.StateFile)
}

func SaveCfg(cfg, ip, context string) error {
	replacer := strings.NewReplacer(
		"127.0.0.1", ip,
		"localhost", ip,
		"default", context,
	)

	result := replacer.Replace(cfg)

	tempPath := fmt.Sprintf("%s/.kube", common.CfgPath)
	if err := utils.EnsureFolderExist(tempPath); err != nil {
		return fmt.Errorf("[cluster] generate kubecfg temp folder error, msg: %s", err)
	}

	temp, err := ioutil.TempFile(tempPath, common.KubeCfgTempName)
	if err != nil {
		return fmt.Errorf("[cluster] generate kubecfg temp file error, msg: %s", err)
	}
	defer func() {
		_ = temp.Close()
	}()

	err = utils.WriteBytesToYaml([]byte(result), tempPath, temp.Name()[strings.Index(temp.Name(), common.KubeCfgTempName):])
	if err != nil {
		return fmt.Errorf("[cluster] write content to kubecfg temp file error, msg: %s", err)
	}

	return mergeCfg(context, temp.Name())
}

func OverwriteCfg(context string) error {
	c, err := clientcmd.LoadFromFile(fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
	if err != nil {
		return err
	}

	delete(c.Clusters, context)
	delete(c.Contexts, context)
	delete(c.AuthInfos, context)

	return clientcmd.WriteToFile(*c, fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile))
}

func DeployExtraManifest(cluster *types.Cluster, cmds []string) error {
	if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, false, cmds); err != nil {
		return err
	}
	return nil
}

func GetK3sNodeInfo(nodeNames string, cluster *types.Cluster, master types.Node, ccmEnabled bool) ([]string, []types.Node, error) {
	var (
		nodes          []types.Node
		validNodeNames []string
	)
	nodeNames = "'" + strings.Replace(nodeNames, ",", "\\|", -1) + "'"
	cmdResult, err := execute(&hosts.Host{Node: master}, true, []string{fmt.Sprintf(getNodeCommand, nodeNames)})
	if err != nil {
		return validNodeNames, nodes, err
	}
	if cmdResult != "" {
		// one line for one node
		lines := strings.Split(cmdResult, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			// nodeName role label
			nodeInfo := strings.Split(line, " ")
			validNodeNames = append(validNodeNames, nodeInfo[0])
			matchedNode := false
			if ccmEnabled {
				if strings.Contains(nodeInfo[1], common.K3sRoleMasterValue) {
					for _, masterNode := range cluster.MasterNodes {
						// <region-id>.<instance-id>
						if strings.Contains(nodeInfo[0], masterNode.InstanceID) {
							nodes = append(nodes, masterNode)
							matchedNode = true
							break
						}
						if matchedNode {
							break
						}
					}
				} else {
					for _, workerNode := range cluster.WorkerNodes {
						if strings.Contains(nodeInfo[0], workerNode.InstanceID) {
							nodes = append(nodes, workerNode)
							matchedNode = true
							break
						}
						if matchedNode {
							break
						}
					}
				}
			} else {
				// labelKey=labelValue
				labels := strings.Split(nodeInfo[2], ",")
				for _, label := range labels {
					kv := strings.Split(label, "=")
					if kv[0] == common.K3sTagNameInternalIP {
						if strings.Contains(nodeInfo[1], common.K3sRoleMasterValue) {
							for _, masterNode := range cluster.MasterNodes {
								for _, internalIPAddress := range masterNode.InternalIPAddress {
									if internalIPAddress == kv[1] {
										nodes = append(nodes, masterNode)
										matchedNode = true
										break
									}
								}
								if matchedNode {
									break
								}
							}
						} else {
							for _, workerNode := range cluster.WorkerNodes {
								for _, internalIPAddress := range workerNode.InternalIPAddress {
									if internalIPAddress == kv[1] {
										nodes = append(nodes, workerNode)
										matchedNode = true
										break
									}
								}
								if matchedNode {
									break
								}
							}
						}
					}
					if matchedNode {
						break
					}
				}
			}
		}
	} else {
		return validNodeNames, nodes, fmt.Errorf("[cluster] specific nodes not found")
	}

	return validNodeNames, nodes, nil
}

func RemoveK3sNode(clusterName string, nodeNames []string, f bool, master types.Node, updateConfig bool) ([]string, error) {
	var (
		messages     []string
		errNodeNames []string
		command      string
	)
	for _, nodeName := range nodeNames {
		command = deleteNodeCommand
		if f {
			command += " --force"
		}
		cmdResult, err := execute(&hosts.Host{Node: master}, true, []string{fmt.Sprintf(command, nodeName)})
		if err != nil {
			errNodeNames = append(errNodeNames, nodeName)
		} else {
			messages = append(messages, nodeName+":"+cmdResult)
		}
	}
	if len(errNodeNames) == 0 {
		if updateConfig {
			// get k3s cluster config.
			cfg, err := execute(&hosts.Host{Node: master}, true, []string{catCfgCommand})
			if err != nil {
				return messages, err
			}
			// merge current cluster to kube config.
			if err := SaveCfg(cfg, master.PublicIPAddress[0], clusterName); err != nil {
				return messages, err
			}
		}
		return messages, nil
	}
	return messages, fmt.Errorf("[cluster] error when delete node [%s] in %s", strings.Join(errNodeNames, ","), clusterName)
}

func initMaster(k3sScript, k3sMirror, dockerMirror, ip, extraArgs string, cluster *types.Cluster, master types.Node) error {
	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: master}, false,
			[]string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			return err
		}
	}

	if _, err := execute(&hosts.Host{Node: master}, false,
		[]string{fmt.Sprintf(initCommand, k3sScript, k3sMirror, cluster.Registries, cluster.Token, "--cluster-init", ip,
			strings.TrimSpace(extraArgs), cluster.K3sVersion)}); err != nil {
		return err
	}

	return nil
}

func initAdditionalMaster(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, ip, extraArgs string,
	cluster *types.Cluster, master types.Node) {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: master}, false,
			[]string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	if !strings.Contains(extraArgs, "server --server") {
		sortedExtraArgs += fmt.Sprintf(" server --server %s --tls-san %s", fmt.Sprintf("https://%s:6443", ip), master.PublicIPAddress[0])
	}

	sortedExtraArgs += " " + extraArgs

	if _, err := execute(&hosts.Host{Node: master}, false,
		[]string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.Registries, ip, cluster.Token,
			strings.TrimSpace(sortedExtraArgs), cluster.K3sVersion)}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func initWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	cluster *types.Cluster, worker types.Node) {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: worker}, false,
			[]string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	sortedExtraArgs += " " + extraArgs

	if _, err := execute(&hosts.Host{Node: worker}, false,
		[]string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.Registries, cluster.IP, cluster.Token,
			strings.TrimSpace(sortedExtraArgs), cluster.K3sVersion)}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func joinMaster(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror,
	extraArgs string, merged *types.Cluster, full types.Node) {
	sortedExtraArgs := ""

	if !strings.Contains(extraArgs, "server --server") {
		sortedExtraArgs += fmt.Sprintf(" server --server %s --tls-san %s", fmt.Sprintf("https://%s:6443", merged.IP), full.PublicIPAddress[0])
	}

	if merged.DataStore != "" {
		sortedExtraArgs += " --datastore-endpoint " + merged.DataStore
	}

	if merged.ClusterCIDR != "" {
		sortedExtraArgs += " --cluster-cidr " + merged.ClusterCIDR
	}

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: full}, false,
			[]string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	sortedExtraArgs += " " + extraArgs

	// for now, use the workerCommand to join the additional master server node.
	if _, err := execute(&hosts.Host{Node: full}, false,
		[]string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.Registries, merged.IP, merged.Token,
			strings.TrimSpace(sortedExtraArgs), merged.K3sVersion)}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func joinWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	merged *types.Cluster, full types.Node) {
	sortedExtraArgs := ""

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: full}, false,
			[]string{fmt.Sprintf(dockerCommand, dockerMirror)}); err != nil {
			errChan <- err
		}
	}

	sortedExtraArgs += " " + extraArgs

	if _, err := execute(&hosts.Host{Node: full}, false,
		[]string{fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.Registries, merged.IP, merged.Token,
			strings.TrimSpace(sortedExtraArgs), merged.K3sVersion)}); err != nil {
		errChan <- err
	}

	wg.Done()
}

func execute(host *hosts.Host, print bool, cmds []string) (string, error) {
	if len(cmds) <= 0 {
		return "", nil
	}

	dialer, err := hosts.SSHDialer(host)
	if err != nil {
		return "", err
	}

	tunnel, err := dialer.OpenTunnel(true)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tunnel.Close()
	}()

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)

	for _, cmd := range cmds {
		tunnel.Cmd(cmd)
	}

	if err := tunnel.SetStdio(&stdout, &stderr).Run(); err != nil {
		return stderr.String(), err
	}

	if print {
		return stdout.String(), nil
	}

	return "", nil
}

func terminal(host *hosts.Host) error {
	dialer, err := hosts.SSHDialer(host)
	if err != nil {
		return err
	}

	tunnel, err := dialer.OpenTunnel(false)
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Close()
	}()

	return tunnel.Terminal()
}

func mergeCfg(context, right string) error {
	defer func() {
		if err := os.Remove(right); err != nil {
			logrus.Errorf("[cluster] remove kubecfg temp file error, msg: %s", err)
		}
	}()

	if err := utils.EnsureFileExist(common.CfgPath, common.KubeCfgFile); err != nil {
		return fmt.Errorf("[cluster] ensure kubecfg exist error, msg: %s", err)
	}

	if err := OverwriteCfg(context); err != nil {
		return fmt.Errorf("[cluster] overwrite kubecfg error, msg: %s", err)
	}

	if err := os.Setenv(clientcmd.RecommendedConfigPathEnvVar, fmt.Sprintf("%s:%s",
		fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile), right)); err != nil {
		return fmt.Errorf("[cluster] set env error when merging kubecfg, msg: %s", err)
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
		return fmt.Errorf("[cluster] generate view options error, msg: %s", err)
	}
	opt.PrintObject = printer.PrintObj

	if err := opt.Run(); err != nil {
		return fmt.Errorf("[cluster] merging kubecfg error, msg: %s", err)
	}

	return utils.WriteBytesToYaml(out.Bytes(), common.CfgPath, common.KubeCfgFile)
}

func deleteClusterFromState(name string, provider string) ([]types.Cluster, error) {
	v := common.CfgPath
	if v == "" {
		return nil, errors.New("[cluster] cfg path is empty")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return nil, err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		return nil, fmt.Errorf("[cluster] failed to unmarshal state file, msg: %s", err)
	}

	index := -1

	for i, c := range converts {
		if c.Provider == provider && c.Name == name {
			index = i
		}
	}

	if index > -1 {
		converts = append(converts[:index], converts[index+1:]...)
	} else {
		return nil, fmt.Errorf("[cluster] was not found in the .state file")
	}

	return converts, nil
}
