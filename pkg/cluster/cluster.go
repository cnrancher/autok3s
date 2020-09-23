package cluster

import (
	"bytes"
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
	initCommand     = "curl -sLS %s | %s INSTALL_K3S_REGISTRIES='%s' K3S_TOKEN='%s' INSTALL_K3S_EXEC='server %s --tls-san %s %s' INSTALL_K3S_VERSION='%s' sh -\n"
	joinCommand     = "curl -sLS %s | %s INSTALL_K3S_REGISTRIES='%s' K3S_URL='https://%s:6443' K3S_TOKEN='%s' INSTALL_K3S_EXEC='%s' INSTALL_K3S_VERSION='%s' sh -\n"
	catCfgCommand   = "cat /etc/rancher/k3s/k3s.yaml"
	dockerCommand   = "curl https://get.docker.com | VERSION=19.03 sh -s - %s\n"
	deployUICommand = "echo \"%s\" > \"%s/ui.yaml\""
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
		close(masterErrChan)
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
		close(workerErrChan)
		return err
	}

	// get k3s cluster config.
	cfg, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, catCfgCommand, false)
	if err != nil {
		return err
	}

	logrus.Infof("[%s] deploying additional manifests\n", cluster.Provider)

	// deploy additional UI manifests. e.g. (none/dashboard/octopus-ui).
	switch cluster.UI {
	case "dashboard":
		if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]},
			fmt.Sprintf(deployUICommand, fmt.Sprintf(dashboardTmpl, cluster.Repo), common.K3sManifestsDir), false); err != nil {
			return err
		}
	case "octopus-ui":
		if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]},
			fmt.Sprintf(deployUICommand, octopusTmpl, common.K3sManifestsDir), false); err != nil {
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
	defer close(errChan)

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
		close(errChan)
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

func DeployExtraManifest(cluster *types.Cluster, deployCommand, tmpl string) error {
	if _, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]},
		fmt.Sprintf(deployCommand, tmpl, common.K3sManifestsDir), false); err != nil {
		return err
	}
	return nil
}

func initMaster(k3sScript, k3sMirror, dockerMirror, ip, extraArgs string, cluster *types.Cluster, master types.Node) error {
	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: master},
			fmt.Sprintf(dockerCommand, dockerMirror), false); err != nil {
			return err
		}
	}

	if _, err := execute(&hosts.Host{Node: master},
		fmt.Sprintf(initCommand, k3sScript, k3sMirror, cluster.Registries, cluster.Token, "--cluster-init", ip,
			strings.TrimSpace(extraArgs), cluster.K3sVersion), false); err != nil {
		return err
	}

	return nil
}

func initAdditionalMaster(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, ip, extraArgs string,
	cluster *types.Cluster, master types.Node) {

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: master},
			fmt.Sprintf(dockerCommand, dockerMirror), false); err != nil {
			errChan <- err
		}
	}

	if !strings.Contains(extraArgs, "server --server") {
		extraArgs += " server --server " + fmt.Sprintf("https://%s:6443", ip)
	}

	if _, err := execute(&hosts.Host{Node: master},
		fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.Registries, ip, cluster.Token,
			strings.TrimSpace(extraArgs), cluster.K3sVersion), false); err != nil {
		errChan <- err
	}

	wg.Done()
}

func initWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	cluster *types.Cluster, worker types.Node) {

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: worker},
			fmt.Sprintf(dockerCommand, dockerMirror), false); err != nil {
			errChan <- err
		}
	}

	if _, err := execute(&hosts.Host{Node: worker},
		fmt.Sprintf(joinCommand, k3sScript, k3sMirror, cluster.Registries, cluster.IP, cluster.Token,
			strings.TrimSpace(extraArgs), cluster.K3sVersion), false); err != nil {
		errChan <- err
	}

	wg.Done()
}

func joinMaster(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror,
	extraArgs string, merged *types.Cluster, full types.Node) {

	if !strings.Contains(extraArgs, "server --server") {
		extraArgs += " server --server " + fmt.Sprintf("https://%s:6443", merged.IP)
	}

	if merged.DataStore != "" {
		extraArgs += " --datastore-endpoint " + merged.DataStore
	}

	if merged.ClusterCIDR != "" {
		extraArgs += " --cluster-cidr " + merged.ClusterCIDR
	}

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: full},
			fmt.Sprintf(dockerCommand, dockerMirror), false); err != nil {
			errChan <- err
		}
	}

	// for now, use the workerCommand to join the additional master server node.
	if _, err := execute(&hosts.Host{Node: full},
		fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.Registries, merged.IP, merged.Token,
			strings.TrimSpace(extraArgs), merged.K3sVersion), false); err != nil {
		errChan <- err
	}

	wg.Done()
}

func joinWorker(wg *sync.WaitGroup, errChan chan error, k3sScript, k3sMirror, dockerMirror, extraArgs string,
	merged *types.Cluster, full types.Node) {

	if strings.Contains(extraArgs, "--docker") {
		if _, err := execute(&hosts.Host{Node: full},
			fmt.Sprintf(dockerCommand, dockerMirror), false); err != nil {
			errChan <- err
		}
	}

	if _, err := execute(&hosts.Host{Node: full},
		fmt.Sprintf(joinCommand, k3sScript, k3sMirror, merged.Registries, merged.IP, merged.Token,
			strings.TrimSpace(extraArgs), merged.K3sVersion), false); err != nil {
		errChan <- err
	}

	wg.Done()
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
		fmt.Printf("[dialer] execute command result:\n %s\n", result)
	}

	return result, nil
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
