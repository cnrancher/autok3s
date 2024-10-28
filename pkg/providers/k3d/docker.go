package k3d

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	"github.com/k3d-io/k3d/v5/pkg/runtimes/docker"
	"github.com/sirupsen/logrus"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	dockerHost string
	once       sync.Once
)

func getDockerHost() string {
	once.Do(func() {
		var err error
		if runtimes.SelectedRuntime.ID() != "docker" {
			logrus.Debugf("runtime not docker")
			return
		}
		// TODO find a better way to get container id via docker.sock
		// Hostname as container id is not 100% reliable.
		hostname := os.Getenv("HOSTNAME_OVERRIDE")
		if hostname == "" {
			hostname, err = os.Hostname()
			if err != nil {
				logrus.Debugf("failed to get hostname, %v", err)
				return
			}
		}

		runtime := runtimes.Docker

		if runtime.GetHost() != "" {
			logrus.Debugf("runtime host is %s", runtime.GetHost())
			return
		}

		if _, err = os.Stat(runtime.GetRuntimePath()); err != nil {
			logrus.Debugf("runtime path %s doesn't exist", runtime.GetRuntimePath())
			return
		}
		nodes, err := getContainersByLabel(context.Background(), map[string]string{
			"org.opencontainers.image.title": "autok3s",
		})
		if err != nil {
			logrus.Debugf("failed to get container from runtime %s, %v", runtime.ID(), err)
			return
		}
		if len(nodes) == 0 {
			logrus.Debug("autok3s docker container not found. Skip finding docker host IP.")
			return
		}
		var currentContainer *types.Container
		for i := range nodes {
			node := nodes[i]
			if strings.HasPrefix(node.ID, hostname) {
				currentContainer = &node
				break
			}
		}
		if currentContainer == nil {
			logrus.Debugf("no container found for hostname %s", hostname)
			return
		}
		if currentContainer.HostConfig.NetworkMode == "host" {
			logrus.Debug("do nothing when running host network")
			return
		}
		logrus.Debugf("found container %s", currentContainer.ID)
		gw, err := runtime.GetHostIP(context.Background(), currentContainer.HostConfig.NetworkMode)
		if err != nil {
			logrus.Debugf("failed to get gateway ip for network %s, %v", currentContainer.HostConfig.NetworkMode, err)
			return
		}
		dockerHost = gw.String()
		logrus.Infof("found docker host IP %s", dockerHost)
	})
	return dockerHost
}

func getContainersByLabel(ctx context.Context, labels map[string]string) ([]types.Container, error) {
	// (0) create docker client
	docker, err := docker.GetDockerClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to create docker client. %+v", err)
	}
	defer docker.Close()

	filters := filters.NewArgs()
	for k, v := range labels {
		filters.Add("label", fmt.Sprintf("%s=%s", k, v))
	}

	containers, err := docker.ContainerList(ctx, container.ListOptions{
		Filters: filters,
		All:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

func OverrideK3dKubeConfigServer(from, to string, config *clientcmdapi.Config) {
	if to == "" {
		return
	}
	if from == "" && getDockerHost() != "" {
		from = getDockerHost()
	}
	for key := range config.Clusters {
		cluster := config.Clusters[key]
		serverURL, _ := url.Parse(cluster.Server)
		if serverURL.Hostname() == from {
			_, port, _ := net.SplitHostPort(serverURL.Host)
			serverURL.Host = fmt.Sprintf("%s:%s", to, port)
		}
		cluster.Server = serverURL.String()
		return
	}
}
