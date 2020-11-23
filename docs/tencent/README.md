# Tencent Provider
It uses the Tencent Cloud SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host. You can also use it to join hosts as masters/agents to the K3s cluster.

## Pre-Requests
The following demo uses the `tencent` Provider, so you need to set the following [RAMs](../tencent/ram.md).
**Security group config:**
Make sure that your security-group allowed port 22(ssh default),6443(kubectl default),8999(if enable ui).

## Usage
User can get the flags available for tencent providers according to the `autok3s <sub-command> --provider tencent --help`.

**ENABLE CCM**
When enabling CCM, if you customize the CIDR of the cluster, you may also need to create a routing table so that the POD can communicate with the VPC normally.
You can create routing table manually from the Tencent Cloud console, or by the [route-ctl](https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl).
> NOTE: The cluster route will **NOT** release automatically. You need to manually delete the cluster route after deleting cluster.

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
