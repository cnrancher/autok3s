# Alibaba Provider
It uses the Alibaba Cloud SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host. You can also use it to join hosts as masters/agents to the K3s cluster.

## Pre-Requests
The following demo uses the `alibaba` Provider, so you need to set the following [RAMs](ram.md).
**Security group config:**
Inbound rules for k3s Server Nodes.

Protocol |  Port  | Source | Description
---|---|---|---|
TCP | 22 | all nodes | for ssh
TCP | 6443 | k3s agent nodes | kubernetes API
TCP | 10250 | k3s server and agent | kubelet
TCP | 8999 | k3s dashboard | (Optional)Required only for dashboard ui
UDP | 8472 | k3s server and agent | (Optional)Required only for Flannel VXLAN
TCP | 2379, 2380 | k3s server nodes | (Optional)Required only for embedded etcd

Typically all outbound traffic is allowed.

## Usage

### Quick Start

Export your credentials into your shell environment and quick start with:
```bash
export ECS_ACCESS_KEY_ID='<Your access key ID>'
export ECS_ACCESS_KEY_SECRET='<Your secret access key>'

autok3s create -p alibaba --name myk3s --master 1 --worker 1
```

OR

```
autok3s create -p alibaba --access-key <access-key> --access-secret <access-secret> --name myk3s --master 1 --worker 1
```

### Options

User can get the flags available for alibaba providers according to the `autok3s <sub-command> --provider alibaba --help`.

CLI | ENV | Description | Required | Default
---|---|---|---|---|
--access-key | ECS_ACCESS_KEY_ID | access key for aliyun API | yes |
--access-secret | ECS_ACCESS_KEY_SECRET | secret key for aliyun API | yes |
--name | | k3s cluster name | yes |
--region | ECS_REGION | aliyun ECS region | no | cn-hangzhou
--zone | ECS_ZONE | aliyun ECS zone | no | cn-hangzhou-e
--key-pair | ECS_SSH_KEYPAIR | aliyun ECS ssh key-pair | no |
--image | ECS_IMAGE_ID | image for ECS instance | no | ubuntu_18_04_x64_20G_alibase_20200618.vhd
--type | ECS_INSTANCE_TYPE | ECS instance type | no | ecs.c6.large
--v-switch | ECS_VSWITCH_ID | ECS vswitch id | no | autok3s-aliyun-vswitch
--disk-category | ECS_DISK_CATEGORY | ECS disk category for instance，`cloud_efficiency` or `cloud_ssd` | no | cloud_ssd
--disk-size | ECS_SYSTEM_DISK_SIZE | ECS system disk size | no | 40GB
--security-group | ECS_SECURITY_GROUP | ECS security group | no | autok3s
--cloud-controller-manager | | enable cloud controller manager | no | false
--master-extra-args | | k3s master extra args | no |
--worker-extra-args | | k3s worker extra args | no |
--registries | | default registry | no |
--datastore | | k3s datastore（Only required for external database with HA mode）| no |
--token | | k3s master token | no |
--master | | master count | yes | 0
--worker | | worker count | yes | 0
--repo | | helm repo | no |
--terway | | enable terway CNI | no | false
--terway-max-pool-size | | Max pool size for terway ENI mode | no | 5
--ui | | enable dashboard ui | no | false

### Setup K3s Cluster
Setup a k3s cluster
```bash
autok3s create \
    --provider alibaba \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret> \
    --master 1
```

HA(embedded etcd: >= 1.19.1-k3s1) mode need `--master` at least 3 master nodes, e.g.
```bash
autok3s ... \
    --master 3
```

HA(external database) mode need `--master` greater than 1 node, also need to specify `--datastore`, e.g.
```bash
autok3s ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join K3s Nodes
Join nodes to specified k3s cluster.
```bash
autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --worker 1
```

Join master nodes to (embedded etcd: >= 1.19.1-k3s1) HA cluster e.g.
```bash
autok3s ... \
    --master 2
```

Join master nodes to (external database) HA cluster, also need to specify `--datastore`, e.g.
```bash
autok3s ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Start K3s Cluster
To start all instance(s) of a k3s cluster.
```bash
autok3s start \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

### Stop K3s Cluster
To stop all instance(s) of a k3s cluster
```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

### Delete K3s Cluster
Remove a specified k3s cluster.
```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

### List K3s Clusters
This command will list the clusters that you have created on this machine.
```bash
autok3s list
```

### Access K3s Cluster
After the cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.
```bash
autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.
```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

### SSH K3s Cluster's Node
Login to specified k3s cluster node via ssh.

```bash
autok3s ssh \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

## Advanced Usage
Autok3s integration some advanced components related to the current provider, e.g. terway/ccm/ui.

### Enable Alibaba Terway CNI Plugin
The instance's type determines the number of EIPs that a K3S cluster can assign to a cluster POD, more detail see [here](https://www.alibabacloud.com/help/zh/doc-detail/97467.htm).

```bash
autok3s create \
    ... \
    --terway "eni"
```

### Enable Alibaba Cloud Controller Manager

```bash
autok3s create \
    ... \
    --cloud-controller-manager
```

### Enable UI Component
This flags will enable [kubernetes/dashboard](https://github.com/kubernetes/dashboard) UI component.
Please following this [docs](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) to create user token.

```bash
autok3s create \
    ... \
    --ui
```
