# K3d Provider

## Introduction

This article provides users with the instructions to create and launch a K3d cluster, and to add nodes for an existing K3d cluster. In addition, this article provides guidance of advanced usages of running K3s on K3d, such as setting up private registry.

## Requirements

- Linux/Unix Environment (Highly Recommend)
- Docker

## Creating a K3d cluster

Please use `autok3s create` command to create a cluster in your local machine.

### Normal Cluster

The following command uses K3d as provider, creates a K3d cluster named "myk3s", and assign it with 1 master node and 1 worker node:

```bash
autok3s -d create -p k3d --name myk3s --master 1 --worker 1
```

### HA Cluster

Please use one of the following commands to create an HA cluster.

#### Embedded etcd

The following command uses K3d as provider, creates an HA K3d cluster named "myk3s", and assigns it with 3 master nodes.

```bash
autok3s -d create -p k3d --name myk3s --master 3
```

## Join K3d Nodes

Please use `autok3s join` command to add one or more nodes for an existing K3s cluster.

### Normal Cluster

The command below shows how to add a worker node for an existing K3d cluster named "myk3s".

```bash
autok3s -d join --provider k3d --name myk3s --worker 1
```

### HA Cluster

The commands to add one or more nodes for an existing HA K3d cluster varies based on the types of HA cluster. Please choose one of the following commands to run.

```bash
autok3s -d join --provider k3d --name myk3s --master 2 --worker 1
```

## Delete K3d Cluster

This command will delete a k3d cluster named "myk3s".

```bash
autok3s -d delete --provider k3d --name myk3s
```

## List K3d Clusters

This command will list the clusters that you have created on this machine.

```bash
autok3s list
```

```bash
NAME   REGION  PROVIDER  STATUS   MASTERS  WORKERS    VERSION     
myk3s          k3d       Running  1        1        v1.20.5+k3s1 
```

## Describe k3d cluster

This command will show detail information of a specified cluster, such as instance status, node IP, kubelet version, etc.

```bash
autok3s describe -n myk3s -p k3d
```

> Note：There will be multiple results if using the same name to create with different providers, please use `-p <provider>` to choose a specified cluster. i.e. `autok3s describe cluster myk3s -p k3d`

```bash
Name: myk3s
Provider: k3d
Region: 
Zone: 
Master: 1
Worker: 1
Status: Running
Version: v1.20.5+k3s1
Nodes:
  - internal-ip: []
    external-ip: []
    instance-status: running
    instance-id: k3d-myk3s-agent-0
    roles: <none>
    status: Ready
    hostname: k3d-myk3s-agent-0
    container-runtime: containerd://1.4.4-k3s1
    version: v1.20.5+k3s1
  - internal-ip: []
    external-ip: []
    instance-status: running
    instance-id: k3d-myk3s-server-0
    roles: control-plane,master
    status: Ready
    hostname: k3d-myk3s-server-0
    container-runtime: containerd://1.4.4-k3s1
    version: v1.20.5+k3s1
```

## Access K3d Cluster

After the cluster is created, `autok3s` will automatically merge the `kubeconfig` so that you can access the cluster.

```bash
autok3s kubectl config use-context k3d-myk3s
autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.

```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

## SSH K3d Cluster's Node

Login to a specific k3d cluster node via ssh, i.e. myk3s.

```bash
autok3s ssh --provider k3d --name myk3s
```

## Advanced Usages

We integrate some advanced components such as private registries, related to the current provider.

### Setting up Private Registry

Registry will only work with K3s >= v0.10.0, secure registry need bind mount the TLS file to the K3d container, more details please see [here](https://k3d.io/usage/guides/registries/#secure-registries).

Below are examples showing how you may configure `/etc/autok3s/registries.yaml` on your current node when using TLS, and make it take effect on k3d cluster by `autok3s`.

```bash
mirrors:
  my.company.registry:
    endpoint:
      - https://my.company.registry

configs:
  my.company.registry:
    tls:
      # we will mount "my-company-root.pem" in the /etc/ssl/certs/ directory.
      ca_file: "/etc/ssl/certs/my-company-root.pem"
```

When running `autok3s create` command, it will take effect with the`--registry /etc/autok3s/registries.yaml` flag, i.e:

```bash
autok3s -d create \
    --provider k3d \
    --name myk3s \
    --master 1 \
    --worker 1 \
    --registry /etc/autok3s/registries.yaml
    --volumes ${HOME}/.k3d/my-company-root.pem:/etc/ssl/certs/my-company-root.pem
```

