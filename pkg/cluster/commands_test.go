package cluster

import (
	"testing"

	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestGetCommand(t *testing.T) {
	testCluster := &types.Cluster{
		Metadata: types.Metadata{
			Cluster:       true,
			K3sVersion:    "v1.24.3+k3s1",
			InstallScript: "https://get.k3s.io",
			Token:         "dd73df9b22f8ff22be0d17ec36e7267a",
			TLSSans: []string{
				"2.3.4.5",
			},
		},
		Status: types.Status{
			MasterNodes: []types.Node{
				{
					Master:            true,
					PublicIPAddress:   []string{"1.2.3.1"},
					InternalIPAddress: []string{"1.2.3.1"},
				},
				{
					Master:            true,
					PublicIPAddress:   []string{"1.2.3.2"},
					InternalIPAddress: []string{"1.2.3.2"},
				},
				{
					Master:            true,
					PublicIPAddress:   []string{"1.2.3.3"},
					InternalIPAddress: []string{"1.2.3.3"},
				},
			},
			WorkerNodes: []types.Node{
				{
					PublicIPAddress:   []string{"1.2.3.5"},
					InternalIPAddress: []string{"1.2.3.5"},
				},
				{
					PublicIPAddress:   []string{"1.2.3.6"},
					InternalIPAddress: []string{"1.2.3.6"},
				},
			},
		},
	}
	masterCommands := []string{
		"curl -sLS https://get.k3s.io | INSTALL_K3S_EXEC='server --cluster-init --node-external-ip=1.2.3.1 " +
			"--tls-san=1.2.3.1 --tls-san=1.2.3.2 --tls-san=1.2.3.3 --tls-san=2.3.4.5' " +
			"INSTALL_K3S_VERSION='v1.24.3+k3s1' K3S_TOKEN='dd73df9b22f8ff22be0d17ec36e7267a' sh -",
		"curl -sLS https://get.k3s.io | INSTALL_K3S_EXEC='server --node-external-ip=1.2.3.2 " +
			"--server=https://1.2.3.1:6443 --tls-san=1.2.3.1 --tls-san=1.2.3.2 --tls-san=1.2.3.3 --tls-san=2.3.4.5' " +
			"INSTALL_K3S_VERSION='v1.24.3+k3s1' K3S_TOKEN='dd73df9b22f8ff22be0d17ec36e7267a' sh -",
		"curl -sLS https://get.k3s.io | INSTALL_K3S_EXEC='server --node-external-ip=1.2.3.3 " +
			"--server=https://1.2.3.1:6443 --tls-san=1.2.3.1 --tls-san=1.2.3.2 --tls-san=1.2.3.3 --tls-san=2.3.4.5' " +
			"INSTALL_K3S_VERSION='v1.24.3+k3s1' K3S_TOKEN='dd73df9b22f8ff22be0d17ec36e7267a' sh -",
	}
	workerCommands := []string{
		"curl -sLS https://get.k3s.io | INSTALL_K3S_EXEC='--node-external-ip=1.2.3.5' " +
			"INSTALL_K3S_VERSION='v1.24.3+k3s1' K3S_TOKEN='dd73df9b22f8ff22be0d17ec36e7267a' K3S_URL='https://1.2.3.1:6443' sh -",
		"curl -sLS https://get.k3s.io | INSTALL_K3S_EXEC='--node-external-ip=1.2.3.6' " +
			"INSTALL_K3S_VERSION='v1.24.3+k3s1' K3S_TOKEN='dd73df9b22f8ff22be0d17ec36e7267a' K3S_URL='https://1.2.3.1:6443' sh -",
	}
	fixedIP := getFirstAddress(testCluster.Status.MasterNodes[0].PublicIPAddress)
	for i, node := range testCluster.MasterNodes {
		rtn := getCommand(i == 0, fixedIP, testCluster, node, []string{})
		assert.Equal(t, masterCommands[i], rtn)
	}
	for i, node := range testCluster.WorkerNodes {
		rtn := getCommand(false, fixedIP, testCluster, node, []string{})
		assert.Equal(t, workerCommands[i], rtn)
	}

	// testing datastore and network
	testCluster.DataStore = "mysql://root:root@tcp(1.2.3.4:3306)/k3s"
	testCluster.Network = "host-gw"
	expectFirstMasterCommand := "curl -sLS https://get.k3s.io | INSTALL_K3S_EXEC='server " +
		"--datastore-endpoint=mysql://root:root@tcp(1.2.3.4:3306)/k3s " +
		"--flannel-backend=host-gw " +
		"--node-external-ip=1.2.3.1 " +
		"--tls-san=1.2.3.1 --tls-san=1.2.3.2 --tls-san=1.2.3.3 --tls-san=2.3.4.5' " +
		"INSTALL_K3S_VERSION='v1.24.3+k3s1' K3S_TOKEN='dd73df9b22f8ff22be0d17ec36e7267a' sh -"

	assert.Equal(t, expectFirstMasterCommand, getCommand(true, fixedIP, testCluster, testCluster.MasterNodes[0], []string{}))
}
