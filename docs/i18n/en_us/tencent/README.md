# Tencent Provider

English / [简体中文](docs/i18n/zh_cn/tencent/README.md)

## Introduction

This article provides users with the instrcutions to create and launch a K3s cluster on an Tencent CVM instance, and to add nodes for an existing K3s cluster on Tencent CVM instance. In additon, this article provides guidance of advanced usages of running K3s on Tencent CVM, such as setting up private registry, enabling Tencent CCM, and enabling UI components.

## Prerequisites

To ensure that CVM instances can be created and accessed successfully, please follow the instructions below.

### Setting up Environment

Configure the following environment variables as showed below for the host on which you are running `autok3s`.

```bash
export CVM_SECRET_ID='<secret-id>'
export CVM_SECRET_KEY='<secret-key>'
```

### Setting up RAM

This provider needs certain permissions to access Tencent Cloud, so need to create a few RAM policies for your CVM instances:

```json
{
  "version": "2.0",
  "statement": [
    {
      "action": [
        "cvm:RunInstances",
        "cvm:DescribeInstances",
        "cvm:TerminateInstances",
        "cvm:StartInstances",
        "cvm:StopInstances",
        "cvm:DescribeInstancesStatus",
        "cvm:AllocateAddresses",
        "cvm:ReleaseAddresses",
        "cvm:AssociateAddress",
        "cvm:DisassociateAddress",
        "cvm:DescribeAddresses",
        "cvm:DescribeImages"
      ],
      "resource": "*",
      "effect": "allow"
    },
    {
      "action": ["vpc:*"],
      "resource": "*",
      "effect": "allow"
    },
    {
      "action": [
        "tag:AddResourceTag",
        "tag:DescribeResourcesByTags",
        "tag:AttachResourcesTag"
      ],
      "resource": "*",
      "effect": "allow"
    },
    {
      "action": ["ccs:Describe*", "ccs:CreateClusterRoute"],
      "resource": "*",
      "effect": "allow"
    },
    {
      "action": ["clb:*"],
      "resource": "*",
      "effect": "allow"
    }
  ]
}
```

### Setting up Security Group

The CVM instances need to apply the following **minimum** Security Group Rules:

```bash
Rule        Protocol    Port      Source             Description
InBound     TCP         22        ALL                SSH Connect Port
InBound     TCP         6443      K3s agent nodes    Kubernetes API
InBound     TCP         10250     K3s server & agent Kubelet
InBound     TCP         8999      K3s dashboard      (Optional) Required only for Dashboard UI
InBound     UDP         8472      K3s server & agent (Optional) Required only for Flannel VXLAN
InBound     TCP         2379,2380 K3s server nodes   (Optional) Required only for embedded ETCD
OutBound    ALL         ALL       ALL                Allow All
```

## Creating a K3s cluster

### Normal Cluster

The following command uses Tencent as cloud provider, creates a K3s cluster named "myk3s", and assign it with 1 master node and 1 worker node:

```bash
autok3s -d create -p tencent --name myk3s --master 1 --worker 1
```

### HA Cluster

Please use one of the following commands to create an HA cluster.

#### Embedded etcd

The following command uses Tencent as cloud provider, creates an HA K3s cluster named "myk3s", and assigns it with 3 master nodes.

```bash
autok3s -d create -p tencent --name myk3s --master 3 --cluster
```

#### External Database

The following requirements must be met before creating an HA K3s cluster with external database:

- The number of master nodes in this cluster must be greater or equal to 1.
- The external database information must be specified within `--datastore "PATH"` parameter.

In the example below, `--master 2` specifies the number of master nodes to be 2, while `--datastore "PATH"` specifies the external database information. As a result, requirements listed above are met.

Run the command below and create an HA K3s cluster with external database:

```bash
autok3s -d create -p tencent --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

## Join K3s Nodes

Please use `autok3s join` command to add one or more nodes for an existing K3s cluster.

### Normal Cluster

The command below shows how to add a worker node for an existing K3s cluster named "myk3s".

```bash
autok3s -d join --provider tencent --name myk3s --worker 1
```

### HA Cluster

The commands to add one or more nodes for an existing HA K3s cluster varies based on the types of HA cluster. Please choose one of the following commands to run.

#### Embedded etcd

Run the command below, to add 2 master nodes for an Embedded etcd HA cluster(embedded etcd: >= 1.19.1-k3s1).

```bash
autok3s -d join --provider tencent --name myk3s --master 2
```

#### External Database

Run the command below, to add 2 master nodes for an HA cluster with external database, you will need to fill in `--datastore "PATH"` as well.

```bash
autok3s -d join --provider tencent --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

## Delete K3s Cluster

This command will delete a k3s cluster named "myk3s".

```bash
autok3s -d delete --provider tencent --name myk3s
```

## List K3s Clusters

This command will list the clusters that you have created on this machine.

```bash
autok3s list
```

```bash
NAME     REGION     PROVIDER  STATUS   MASTERS  WORKERS    VERSION
myk3s  cn-hangzhou  alibaba   Running  2        2        v1.19.5+k3s2
myk3s  ap-nanjing   tencent   Running  2        1        v1.19.5+k3s2
```

## Describe k3s cluster

This command will show detail information of specified cluster, such as instance status, node IP, kubelet version, etc.

```bash
autok3s describe cluster <clusterName>
```

> Note：There will be multiple results if using the same name to create with different providers, please use `-p <provider> -r <region>` to choose specified cluster. e.g. `autok3s describe cluster <clusterName> -p tencent -r <region>`

```bash
Name: myk3s
Provider: tencent
Region: ap-nanjing
Zone: ap-nanjing-1
Master: 2
Worker: 1
Status: Running
Version: v1.19.5+k3s2
Nodes:
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: RUNNING
    instance-id: xxxxx
    roles: etcd,master
    status: Ready
    hostname: xxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: RUNNING
    instance-id: xxxxx
    roles: <none>
    status: Ready
    hostname: xxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: RUNNING
    instance-id: xxxxxx
    roles: etcd,master
    status: Ready
    hostname: xxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
```

## Access K3s Cluster

After the cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.

```bash
autok3s kubectl config use-context myk3s.ap-guangzhou.tencent
autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.

```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

## SSH K3s Cluster's Node

Login to specified k3s cluster node via ssh, e.g myk3s.

```bash
autok3s ssh --provider tencent --name myk3s
```

## Other Usages

More usage details please running `autok3s <sub-command> --provider tencent --help` commands.

## Advanced Usages

### Setting up Private Registry

Below are examples showing how you may configure `/etc/autok3s/registries.yaml` on your current node when using TLS, and make it take effect on k3s cluster by `autok3s`.

```bash
mirrors:
  docker.io:
    endpoint:
      - "https://mycustomreg.com:5000"
configs:
  "mycustomreg:5000":
    auth:
      username: xxxxxx # this is the registry username
      password: xxxxxx # this is the registry password
    tls:
      cert_file: # path to the cert file used in the registry
      key_file:  # path to the key file used in the registry
      ca_file:   # path to the ca file used in the registry
```

When running `autok3s create` or `autok3s join` command, take effect with the`--registry /etc/autok3s/registries.yaml` flag, e.g:

```bash
autok3s -d create -p tencent --name myk3s --master 3 --registry /etc/autok3s/registries.yaml
```

### Enable Tencent Cloud Controller Manager

You should create cluster route table if enabled CCM, and set `--router` with you router table name.

Autok3s uses `10.42.0.0/16` as default cluster CIDR, your route table should set the same cidr-block.

Using [route-ctl](https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl) to create cluster route table.

```bash
export QCloudSecretId=************************************
export QCloudSecretKey=********************************
export QCloudCcsAPIRegion=<your-region>

./route-ctl route-table create --route-table-cidr-block 10.42.0.0/16 --route-table-name <your-route-table-name> --vpc-id <your-vpc-id>
```

Then using `<your-route-table-name>` value for `--router`, the `--vpc` should be the same with vpc you set for route table name.

```bash
autok3s -d create \
    ... \
    --cloud-controller-manager --router <your-route-table-name> --vpc <your-vpc-id> --subnet <your-subnet-id>
```

The cluster route table will not **DELETE AUTOMATICALLY**, please remove router with [route-ctl](https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl).

### Enable UI Component

This flags will enable [kubernetes/dashboard](https://github.com/kubernetes/dashboard) UI component.
Please following this [docs](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) to create user token.

```bash
autok3s -d create \
    ... \
    --ui
```
