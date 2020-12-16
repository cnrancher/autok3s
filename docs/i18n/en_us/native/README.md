# Native Provider
It does not integrate the Cloud SDK, but only uses SSH to install or join K3s cluster and hosts.

## Pre-Requests
Provision a new VM running a compatible operating system such as (Ubuntu, Debian, Raspbian, etc) and register or setup `SSH keys/password` for them.

### Setup Security Group
The VM instances need to apply the following Security Group Rules:

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
More usage details please running `autok3s <sub-command> --provider native --help` commands.

### Quick Start
This command will create a k3s cluster, e.g myk3s.

```bash
autok3s -d create \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master0-ip> \
    --worker-ips <worker0-ip,worker1-ip>
```
### Setup K3s HA Cluster
HA(embedded etcd: >= 1.19.1-k3s1) mode need `--master-ips` at least 3, e.g.

```bash
autok3s -d create \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master0-ip,master1-ip,master2-ip>
```

HA(external database) mode need `--master-ips` greater than 1, also need to specify `--datastore`, e.g.

```bash
autok3s -d create \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master0-ip,master1-ip> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join K3s Nodes
To join master/agent nodes, specify the cluster you want to add, e.g myk3s.

```bash
autok3s -d join \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --worker-ips <worker1-ip,worker2-ip>
```


Join master nodes to (embedded etcd: >= 1.19.1-k3s1) HA cluster e.g.

```bash
autok3s -d join \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master1-ip,master2-ip>
```

Join master nodes to (external database) HA cluster, also need to specify `--datastore`, e.g.

```bash
autok3s -d join \
    --provider native \
    --name myk3s \
    --master-ips <master1-ip,master2-ip> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Delete K3s Cluster
This command will delete a k3s cluster, e.g myk3s.

```bash
autok3s delete --provider native --name myk3s
```

### List K3s Clusters
This command will list the clusters that you have created on this machine.

```bash
autok3s list
```

### Access K3s Cluster
After the cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.

```bash
autok3s kubectl config use-context myk3s
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
autok3s ssh --provider native --name myk3s
```
## Advanced Usage
We integrate some advanced components related to the current provider, e.g. ui.

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
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master0-ip> \
    --worker-ips <worker0-ip,worker1-ip> \
    --registry /etc/autok3s/registries.yaml
```

### Enable UI Component
This flags will enable [kubernetes/dashboard](https://github.com/kubernetes/dashboard) UI component.
Please following this [docs](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) to create user token.

```bash
autok3s create \
    ... \
    --ui
```
