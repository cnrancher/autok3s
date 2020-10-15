# Alibaba Provider
It uses the Alibaba Cloud SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host. You can also use it to join hosts as masters/agents to the K3s cluster.

## Pre-Requests
The following demo uses the `alibaba` Provider, so you need to set the following [RAMs](ram.md).

## Usage
User can get the flags available for alibaba providers according to the `autok3s <sub-command> --provider alibaba --help`.

### Setup K3s Cluster
If already have access information in `$HOME/.autok3s/config.yaml` you can use the simplified command.
```bash
autok3s create \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --ssh-key-path <ssh-key-path> \
    --master 1
```

Generic commands can be used anywhere.
```bash
autok3s create \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --ssh-key-path <ssh-key-path> \
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
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --worker 1
```

Generic commands can be used anywhere.
```bash
autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --token <k3s token> \
    --ip <k3s master/lb ip> \
    --access-key <access-key> \
    --access-secret <access-secret> \
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
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s start \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
```

### Stop K3s Cluster
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
```

### Delete K3s Cluster
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
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
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s ssh \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh private key path> \
    --ssh-user root \
    --ssh-port 22 \
    --access-key <access-key> \
    --access-secret <access-secret>
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
