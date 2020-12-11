# Tencent Provider
It uses the Tencent Cloud SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host. You can also use it to join hosts as masters/agents to the K3s cluster.

## Pre-Requests
To ensure that CVM instances can be created and accessed normally, please check and set the following configuration.

### Setup Environment
Configure the following environment variables for the host which running `autok3s`.

```bash
export CVM_SECRET_ID='<secret-id>'
export CVM_SECRET_KEY='<secret-key>'
```

### Setup RAM
This provider needs certain permissions to access Tencent Cloud, so need to create a few RAM policies for your CMV instances:

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
            "action": [
                "vpc:*"
            ],
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
            "action": [
                "ccs:Describe*",
                "ccs:CreateClusterRoute"
            ],
            "resource": "*",
            "effect": "allow"
        },
        {
            "action": [
                "clb:*"
            ],
            "resource": "*",
            "effect": "allow"
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
More usage details please running `autok3s <sub-command> --provider tencent --help` commands.

### Quick Start
Create and Start 1 master & 1 worker(agent) k3s cluster.
```bash
autok3s -d create -p tencent --name myk3s --master 1 --worker 1 --ssh-user ubuntu
```

### Setup K3s HA Cluster
HA(embedded etcd: >= 1.19.1-k3s1) mode need `--master` at least 3, e.g.

```bash
autok3s -d ... \
    --master 3
```

HA(external database) mode need `--master` greater than 1, also need to specify `--datastore`, e.g.

```bash
autok3s -d ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join K3s Nodes
To join master/agent nodes, specify the cluster you want to add, e.g myk3s.

```bash
autok3s -d join \
    --provider tencent \
    --name myk3s \
    --ssh-user ubuntu \
    --worker 1
```

Join master nodes to (embedded etcd: >= 1.19.1-k3s1) HA cluster e.g.

```bash
autok3s -d ... \
    --master 2
```

Join master nodes to (external database) HA cluster, also need to specify `--datastore`, e.g.

```bash
autok3s -d ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Start K3s Cluster
This command will start a stopped k3s cluster, e.g myk3s.

```bash
autok3s -d start \
    --provider tencent \
    --name myk3s
```

### Stop K3s Cluster
This command will stop a running k3s cluster, e.g myk3s.

```bash
autok3s -d stop \
    --provider tencent \
    --name myk3s
```

### Delete K3s Cluster
This command will delete a k3s cluster, e.g myk3s.

```bash
autok3s -d delete \
    --provider tencent \
    --name myk3s
```

### List K3s Clusters
This command will list the clusters that you have created on this machine.

```bash
autok3s list
```

### Access K3s Cluster
After the cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.

```bash
autok3s kubectl config use-context myk3s.ap-guangzhou
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
autok3s ssh \
    --provider tencent \
    --name myk3s \
    --ssh-user ubuntu
```

## Advanced Usage
Autok3s integration some advanced components related to the current provider, e.g. ccm/ui.

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
