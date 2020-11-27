# Tencent Provider
It uses the Tencent Cloud SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host. You can also use it to join hosts as masters/agents to the K3s cluster.

## Pre-Requests
The following demo uses the `tencent` Provider, so you need to set the following [RAMs](../tencent/ram.md).
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

> NOTE：CVM security group is denied all by default for Egress.

## Usage
User can get the flags available for tencent providers according to the `autok3s <sub-command> --provider tencent --help`.

**ENABLE CCM**
When enabling CCM, if you customize the CIDR of the cluster, 
you may also need to create a routing table so that the POD can communicate with the VPC normally.
For more information, please click [here](#Enable Tencent Cloud Controller Manager)

Export your credentials into your shell environment and quick start with:
```bash
export CVM_SECRET_ID='<Your secret id>'
export CVM_SECRET_KEY='<Your secret key>'

autok3s create -p tencent --name myk3s --master 1 --worker 1 --ssh-user ubuntu
```

OR

```
autok3s create -p tencent --secret-id <secret-id> --secret-key <secret-key> --name myk3s --master 1 --worker 1 --ssh-user ubuntu
```

### Options

User can get the flags available for alibaba providers according to the `autok3s <sub-command> --provider tencent --help`.

CLI | ENV | Description | Required | Default
---|---|---|---|---|
--secret-id | CVM_SECRET_ID | secret id for tencent API | yes |
--secret-key | CVM_SECRET_KEY | secret key for tencent API | yes |
--name | | k3s cluster name | yes |
--region | CVM_REGION | tencent CVM region | no | ap-guangzhou
--zone | CVM_ZONE | tencent CVM zone | no | ap-guangzhou-3
--key-pair | CVM_SSH_KEYPAIR | key-pair for ssh tencent CVM | no |
--image | | image ID for CVM | no | img-pi0ii46r(Ubuntu Server 18.04.1 LTS x64)
--type | CVM_INSTANCE_TYPE | intance type of CVM | no | SA1.MEDIUM4
--vpc | CVM_VPC_ID | tencent vpc ID | no | autok3s-tencent-vpc
--subnet | CVM_SUBNET_ID | tencent subnet ID of specified VPC | no | autok3s-tencent-subnet
--disk-category | CVM_DISK_CATEGORY | disk category of CVM | no | CLOUD_SSD
--disk-size | CVM_DISK_SIZE | system disk size | no | 60GB
--security-group | CVM_SECURITY_GROUP | security group of instance | no | autok3s
--cloud-controller-manager | | enable cloud controller manager | no | false
--master-extra-args | | k3s master extra args | no |
--worker-extra-args | | k3s worker extra args | no |
--registries | | private registries | no |
--datastore | | k3s datastore（Only required for external database with HA mode）| no |
--token | | k3s master token | no |
--master | | master number | yes | 0
--worker | | worker number | yes | 0
--repo | | helm repo | no |
--ui | | enable dashboard ui | no | false
--router | | route table name of vpc, must set when you enabled cloud controller manager | no |

### Setup K3s Cluster
If already have access information in `$HOME/.autok3s/config.yaml` you can use the simplified command.
```bash
autok3s create \
    --provider tencent \
    --region <region> \
    --zone <zone> \
    --name <cluster name> \
    --security-group <security-group id> \
    --vpc <vpc id> \
    --subnet <subnet id> \
    --key-pair <key-pair id> \
    --ssh-user <ssh-user> \
    --ssh-key-path <ssh-key-path> \
    --master 1
```

Generic commands can be used anywhere.
```bash
autok3s create \
    --provider tencent \
    --region <region> \
    --zone <zone> \
    --name <cluster name> \
    --security-group <security-group id> \
    --vpc <vpc id> \
    --subnet <subnet id> \
    --key-pair <key-pair id> \
    --ssh-user <ssh-user> \
    --ssh-key-path <ssh-key-path> \
    --secret-id <secret-id> \
    --secret-key <secret-key> \
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

Enable eip need flag `--eip`, e.g.
```bash
autok3s ... \
    --eip
```

### Join K3s Nodes
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s join \
    --provider tencent \
    --region <region> \
    --zone <zone> \
    --name <cluster name> \
    --ssh-user <ssh-user> \
    --ssh-key-path <ssh-key-path> \
    --worker 1
```

Generic commands can be used anywhere.
```bash
autok3s join \
    --provider tencent \
    --region <region> \
    --zone <zone> \
    --name <cluster name> \
    --security-group <security-group id> \
    --vpc <vpc id> \
    --subnet <subnet id> \
    --key-pair <key-pair id> \
    --ssh-user <ssh-user> \
    --ssh-key-path <ssh-key-path> \
    --token <k3s token> \
    --ip <k3s master/lb ip> \
    --secret-id <secret-id> \
    --secret-key <secret-key> \
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
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s start \
    --provider tencent \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s start \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
```

### Stop K3s Cluster
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s stop \
    --provider tencent \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s stop \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
```

### Delete K3s Cluster
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s delete \
    --provider tencent \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s delete \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
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
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s ssh \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --ssh-user <ssh-user>
```

Generic commands can be used anywhere.
```bash
autok3s ssh \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --ssh-user <ssh-user> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
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
autok3s create \
    ... \
    --cloud-controller-manager --router <your-route-table-name> --vpc <your-vpc-id> --subnet <your-subnet-id>
```

The cluster route table will not **DELETE AUTOMATICALLY**, please remove router with [route-ctl](https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl).

### Enable UI Component
This flags will enable [kubernetes/dashboard](https://github.com/kubernetes/dashboard) UI component.
Please following this [docs](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) to create user token.

```bash
autok3s create \
    ... \
    --ui
```
