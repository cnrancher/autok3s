package cluster

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Jason-ZW/autok3s/pkg/common"
	"github.com/Jason-ZW/autok3s/pkg/hosts"
	"github.com/Jason-ZW/autok3s/pkg/types"
	"github.com/Jason-ZW/autok3s/pkg/utils"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
)

var (
	masterCommand = "curl -sLS https://docs.rancher.cn/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn K3S_TOKEN='%s' INSTALL_K3S_EXEC='--tls-san %s' sh -\n"
	workerCommand = "curl -sLS https://docs.rancher.cn/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn K3S_URL='https://%s:6443' K3S_TOKEN='%s' sh -\n"
	catCfgCommand = "cat /etc/rancher/k3s/k3s.yaml"
)

func InitK3sCluster(cluster *types.Cluster) error {
	token, err := utils.RandomToken(16)
	if err != nil {
		return err
	}

	if len(cluster.MasterNodes) <= 0 || len(cluster.MasterNodes[0].InternalIPAddress) <= 0 {
		return errors.New("[dialer] master node internal ip address can not be empty\n")
	}

	url := cluster.MasterNodes[0].InternalIPAddress[0]
	publicIP := cluster.MasterNodes[0].PublicIPAddress[0]

	for _, master := range cluster.MasterNodes {
		if _, err := execute(&hosts.Host{Node: master}, fmt.Sprintf(masterCommand, token, publicIP), true); err != nil {
			return err
		}
	}

	for _, worker := range cluster.WorkerNodes {
		if _, err := execute(&hosts.Host{Node: worker}, fmt.Sprintf(workerCommand, url, token), true); err != nil {
			return err
		}
	}

	// get k3s cluster config.
	cfg, err := execute(&hosts.Host{Node: cluster.MasterNodes[0]}, catCfgCommand, false)
	if err != nil {
		return err
	}

	if err := writeCfg(cfg, publicIP, cluster.Name); err != nil {
		return err
	}

	// write current cluster to state file.
	return writeState(cluster)
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

func writeState(cluster *types.Cluster) error {
	v := common.CfgPath
	if v == "" {
		return errors.New("cfg path is empty\n")
	}

	clusters, err := utils.ReadYaml(v, common.StateFile)
	if err != nil {
		return err
	}

	converts, err := ConvertToClusters(clusters)
	if err != nil {
		logrus.Fatalf("failed to unmarshal state file, msg: %s\n", err.Error())
	}

	result := make([]types.Cluster, 0)

	for _, c := range converts {
		if c.Provider == cluster.Provider && c.Name == cluster.Name {
			result = append(result, *cluster)
		}
	}

	if len(result) == 0 && cluster != nil {
		result = append(result, *cluster)
	}

	return utils.WriteYaml(result, v, common.StateFile)
}

func writeCfg(cfg, ip, context string) error {
	replacer := strings.NewReplacer(
		"127.0.0.1", ip,
		"localhost", ip,
		"default", context,
	)

	result := replacer.Replace(cfg)

	err := utils.EnsureFileExist(common.CfgPath, common.KubeCfgFile)
	if err != nil {
		return err
	}

	return utils.WriteBytesToYaml([]byte(result), common.CfgPath, common.KubeCfgFile)
}
