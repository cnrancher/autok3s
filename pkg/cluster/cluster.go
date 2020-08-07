package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"

	"github.com/Jason-ZW/autok3s/pkg/common"
	"github.com/Jason-ZW/autok3s/pkg/hosts"
	"github.com/Jason-ZW/autok3s/pkg/types"
	"github.com/Jason-ZW/autok3s/pkg/utils"
)

var (
	masterCommand = "curl -sLS https://docs.rancher.cn/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn K3S_TOKEN='%s' sh -\n"
	workerCommand = "curl -sLS https://docs.rancher.cn/k3s/k3s-install.sh | INSTALL_K3S_MIRROR=cn K3S_URL='https://%s:6443' K3S_TOKEN='%s' sh -\n"
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

	for _, master := range cluster.MasterNodes {
		if err := initK3s(&hosts.Host{Node: master}, fmt.Sprintf(masterCommand, token)); err != nil {
			return err
		}
	}

	for _, worker := range cluster.WorkerNodes {
		if err := initK3s(&hosts.Host{Node: worker}, fmt.Sprintf(workerCommand, url, token)); err != nil {
			return err
		}
	}

	// write current cluster to state file.
	return writeState(cluster)
}

func ConvertToClusters(origin []interface{}) ([]types.Cluster, error) {
	result := make([]types.Cluster, 0)

	b, err := json.Marshal(origin)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func initK3s(host *hosts.Host, cmd string) error {
	dialer, err := hosts.SSHDialer(host)
	if err != nil {
		return err
	}

	tunnel, err := dialer.OpenTunnel()
	if err != nil {
		return err
	}
	defer func() {
		_ = tunnel.Close()
	}()

	result, err := tunnel.ExecuteCommand(cmd)
	if err != nil {
		return err
	}

	fmt.Printf("[dialer] execute command result: %s\n", result)

	return nil
}

func writeState(cluster *types.Cluster) error {
	v := common.CfgFile
	if v == "" {
		return errors.New("cfg path is empty\n")
	}

	p := v[0:strings.LastIndex(v, "/")]

	clusters, err := utils.ReadYaml(p, common.StateFile)
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

	return utils.WriteYaml(result, p, common.StateFile)
}
