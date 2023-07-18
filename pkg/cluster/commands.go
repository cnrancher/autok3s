package cluster

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cnrancher/autok3s/pkg/types"
)

const (
	tlsSanArg = "--tls-san"
)

var (
	getTokenCommand        = "cat /var/lib/rancher/k3s/server/node-token"
	catCfgCommand          = "cat /etc/rancher/k3s/k3s.yaml"
	dockerCommand          = "if ! type docker; then curl -sSL %s | %s sh - %s; fi"
	masterUninstallCommand = "[ -x /usr/local/bin/k3s-uninstall.sh ] && sh /usr/local/bin/k3s-uninstall.sh || true"
	workerUninstallCommand = "[ -x /usr/local/bin/k3s-agent-uninstall.sh ] && sh /usr/local/bin/k3s-agent-uninstall.sh || true"
	k3sRestart             = `if [ -n "$(command -v systemctl)" ]; then systemctl restart k3s; elif [ -n "$(command -v service)" ]; then service k3s restart; fi`
	k3sAgentRestart        = `if [ -n "$(command -v systemctl)" ]; then systemctl restart k3s-agent; elif [ -n "$(command -v service)" ]; then service k3s-agent restart; fi`
)

// getCommand first node should be init
func getCommand(isFirstMaster bool, fixedIP string, cluster *types.Cluster, node types.Node, extraArgs []string) string {
	var commandPrefix, commandSuffix string
	envVar := map[string]string{}
	// airgap install
	if cluster.PackageName != "" || cluster.PackagePath != "" {
		commandSuffix = "install.sh"
		envVar["INSTALL_K3S_SKIP_DOWNLOAD"] = "true"
	} else {
		commandPrefix = fmt.Sprintf("curl -sLS %s |", cluster.InstallScript)
		commandSuffix = "sh -"
		if cluster.K3sVersion != "" {
			envVar["INSTALL_K3S_VERSION"] = cluster.K3sVersion
		} else if cluster.K3sChannel != "" {
			envVar["INSTALL_K3S_CHANNEL"] = cluster.K3sChannel
		}
	}

	if cluster.Mirror != "" {
		kv := strings.SplitN(cluster.Mirror, "=", 2)
		if len(kv) < 2 {
			kv = append(kv, "")
		}
		envVar[kv[0]] = kv[1]
	}
	envVar["K3S_TOKEN"] = cluster.Token

	if !node.Master {
		envVar["K3S_URL"] = fmt.Sprintf("https://%s:6443", fixedIP)
	}

	runArgs := getRunArgs(isFirstMaster, fixedIP, cluster, node)
	runArgs = append(runArgs, extraArgs...)
	envVar["INSTALL_K3S_EXEC"] = strings.Join(runArgs, " ")

	sortedEnvVars := []string{}
	for k, v := range envVar {
		sortedEnvVars = append(sortedEnvVars, fmt.Sprintf("%s='%s'", k, v))
	}
	sort.Strings(sortedEnvVars)
	return strings.TrimSpace(fmt.Sprintf("%s %s %s", commandPrefix, strings.Join(sortedEnvVars, " "), commandSuffix))
}

func getTLSSans(cluster *types.Cluster) []string {
	dedump := map[string]bool{}
	for _, san := range cluster.TLSSans {
		dedump[san] = true
	}
	for _, master := range cluster.MasterNodes {
		if addr := getFirstAddress(master.PublicIPAddress); addr != "" {
			dedump[addr] = true
		}
		if addr := getFirstAddress(master.InternalIPAddress); addr != "" {
			dedump[addr] = true
		}
	}
	var rtn []string
	for k := range dedump {
		rtn = append(rtn, k)
	}
	sort.Strings(rtn)
	return rtn
}

// getFirstAddress
func getFirstAddress(addresses []string) string {
	for _, addr := range addresses {
		if addr != "" {
			return addr
		}
	}
	return ""
}

func getRunArgs(isFirstMaster bool, fixedIP string, cluster *types.Cluster, node types.Node) []string {
	runArgs := []string{}

	// should also handle join command as well
	if node.Master {
		isCluster := cluster.Cluster
		if cluster.DataStore != "" {
			isCluster = false
			runArgs = append(runArgs, "--datastore-endpoint="+cluster.DataStore)
		}

		if cluster.DataStoreCAFileContent != "" || cluster.DataStoreCAFile != "" {
			runArgs = append(runArgs, fmt.Sprintf("--datastore-cafile=%s/ds-ca.pem", datastoreCertificatesPath))
		}

		if cluster.DataStoreCertFileContent != "" || cluster.DataStoreCertFile != "" {
			runArgs = append(runArgs, fmt.Sprintf("--datastore-certfile=%s/ds-cert.pem", datastoreCertificatesPath))
		}

		if cluster.DataStoreKeyFileContent != "" || cluster.DataStoreKeyFile != "" {
			runArgs = append(runArgs, fmt.Sprintf("--datastore-keyfile=%s/ds-key.pem", datastoreCertificatesPath))
		}

		if isCluster && isFirstMaster {
			runArgs = append(runArgs, "--cluster-init")
		}
		if isCluster && !isFirstMaster {
			runArgs = append(runArgs, "--server="+fmt.Sprintf("https://%s:6443", fixedIP))
		}

		if cluster.ClusterCidr != "" {
			runArgs = append(runArgs, "--cluster-cidr="+cluster.ClusterCidr)
		}

		for _, san := range getTLSSans(cluster) {
			runArgs = append(runArgs, tlsSanArg+"="+san)
		}

		internalIPAddress := getFirstAddress(node.InternalIPAddress)
		if internalIPAddress != "" {
			runArgs = append(runArgs, "--advertise-address="+internalIPAddress)
		}
	}

	if externalAddr := getFirstAddress(node.PublicIPAddress); externalAddr != "" {
		runArgs = append(runArgs, "--node-external-ip="+externalAddr)
	}

	if cluster.Network != "" {
		runArgs = append(runArgs, "--flannel-backend="+cluster.Network)
	}

	if cluster.SystemDefaultRegistry != "" {
		runArgs = append(runArgs, "--system-default-registry="+cluster.SystemDefaultRegistry)
	}

	sort.Strings(runArgs)
	if node.Master {
		// ensure server arg is the first one
		runArgs = append([]string{"server"}, runArgs...)
	}
	return runArgs
}
