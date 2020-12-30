# Alibaba Provider
It uses the Alibaba Cloud SDK to create and manage hosts, and then uses SSH to install the k3s cluster to the remote host.
You can also use it to join hosts as masters/agents to the k3s cluster.

## Pre-Requests
To ensure that ECS instances can be created and accessed normally, please check and set the following configuration.

### Setup Environment
Configure the following environment variables for the host which running `autok3s`.

```bash
export ECS_ACCESS_KEY_ID='<access-key>'
export ECS_ACCESS_KEY_SECRET='<secret-access>'
```

### Setup RAM
What is RAM role of an instance, please see [here](https://www.alibabacloud.com/help/doc-detail/54235.htm).

This provider needs certain permissions to access Alibaba Cloud, so need to create a few RAM policies for your ECS instances:

```json
{
  "Version": "1",
  "Statement": [
    {
      "Action": [
        "ecs:Describe*",
        "ecs:AttachDisk",
        "ecs:CreateDisk",
        "ecs:CreateSnapshot",
        "ecs:CreateRouteEntry",
        "ecs:DeleteDisk",
        "ecs:DeleteSnapshot",
        "ecs:DeleteRouteEntry",
        "ecs:DetachDisk",
        "ecs:ModifyAutoSnapshotPolicyEx",
        "ecs:ModifyDiskAttribute",
        "ecs:CreateNetworkInterface",
        "ecs:AttachNetworkInterface",
        "ecs:DetachNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:CreateNetworkInterface",
        "ecs:AttachNetworkInterface",
        "ecs:DetachNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:AssignPrivateIpAddresses",
        "ecs:UnassignPrivateIpAddresses",
        "ecs:DeleteInstances",
        "ecs:RunInstances",
        "ecs:ListTagResources",
        "ecs:StartInstances",
        "ecs:StopInstances"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "cr:Get*",
        "cr:List*",
        "cr:PullRepository"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "slb:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "cms:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "vpc:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "log:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "nas:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    }
  ]
}
```

### Setup Security Group
The ECS instances need to apply the following Security Group Rules:

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

## Usage
More usage details please running `autok3s <sub-command> --provider alibaba --help` commands.

### Quick Start
Create and Start 1 master & 1 worker(agent) k3s cluster.

```bash
autok3s -d create -p alibaba --name myk3s --master 1 --worker 1
```

### Setup K3s HA Cluster
HA(embedded etcd: >= 1.19.1-k3s1) mode. e.g.

```bash
autok3s -d create -p alibaba --name myk3s --master 3 --cluster
```

HA(external database) mode need `--master` greater than 1, also need to specify `--datastore`, e.g.

```bash
autok3s -d create -p alibaba --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join K3s Nodes
To join master/agent nodes, specify the cluster you want to add, e.g myk3s.

```bash
autok3s -d join --provider alibaba --name myk3s --worker 1
```

Join master nodes to (embedded etcd: >= 1.19.1-k3s1) HA cluster.  e.g.

```bash
autok3s -d join --provider alibaba --name myk3s --master 2
```

Join master nodes to (external database) HA cluster, also need to specify `--datastore`, e.g.

```bash
autok3s -d join --provider alibaba --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Start K3s Cluster
This command will start a stopped k3s cluster, e.g myk3s.

```bash
autok3s -d start --provider alibaba --name myk3s
```

### Stop K3s Cluster
This command will stop a running k3s cluster, e.g myk3s.

```bash
autok3s -d stop --provider alibaba --name myk3s
```

### Delete K3s Cluster
This command will delete a k3s cluster, e.g myk3s.

```bash
autok3s -d delete --provider alibaba --name myk3s
```

### List K3s Clusters
This command will list the clusters that you have created on this machine.

```bash
autok3s list
```

```bash
NAME     REGION     PROVIDER  STATUS   MASTERS  WORKERS    VERSION
myk3s  cn-hangzhou  alibaba   Running  2        2        v1.19.5+k3s2
myk3s  ap-nanjing   tencent   Running  2        1        v1.19.5+k3s2
```

### Describe k3s cluster
This command will show detail information of specified cluster, such as instance status, node IP, kubelet version, etc.

```bash
autok3s describe cluster <clusterName>
```
> Note：There will be multiple results if using the same name to create with different providers, please use `-p <provider> -r <region>` to choose specified cluster. e.g. `autok3s describe cluster <clusterName> -p alibaba -r <region>`

```bash
Name: myk3s
Provider: alibaba
Region: cn-hangzhou
Zone: cn-hangzhou-i
Master: 2
Worker: 2
Status: Running
Version: v1.19.5+k3s2
Nodes:
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxx
    roles: etcd,master
    status: Ready
    hostname: xxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxxx
    roles: <none>
    status: Ready
    hostname: xxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxxxxx
    roles: etcd,master
    status: Ready
    hostname: xxxxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxxxx
    roles: <none>
    status: Ready
    hostname: xxxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
```

### Access K3s Cluster
After the cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.

```bash
autok3s kubectl config use-context myk3s.cn-hangzhou.alibaba
autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.

```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

### SSH K3s Cluster's Node
Login to specified k3s cluster node via ssh, e.g myk3s.

```bash
autok3s ssh --provider alibaba --name myk3s
```

## Advanced Usage
We integrate some advanced components related to the current provider, e.g. terway/ccm/ui.

### Setup Private Registry
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
autok3s -d create \
    --provider alibaba \
    --name myk3s \
    --master 1 \
    --worker 1 \
    --registry /etc/autok3s/registries.yaml
```

### Enable Alibaba Terway CNI Plugin
The instance's type determines the number of EIPs that a K3S cluster can assign to a cluster POD, more detail see [here](https://www.alibabacloud.com/help/zh/doc-detail/97467.htm).

```bash
autok3s -d create \
    ... \
    --terway "eni"
```

### Enable Alibaba Cloud Controller Manager
```bash
autok3s -d create \
    ... \
    --cloud-controller-manager
```

### Enable UI Component
This flag will enable [kubernetes/dashboard](https://github.com/kubernetes/dashboard) UI component.
Please following this [docs](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) to create user token.

```bash
autok3s -d create \
    ... \
    --ui
```
