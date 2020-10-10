## Provider native
### Setup K3s Cluster
```bash
autok3s create \
    --provider native \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master0-ip>
    --worker-ips <worker0-ip,worker1-ip>
```

SSH connection also supports the following options.

| Option Name | Description |
| --- | --- |
| --ssh-user | SSH user for host |
| --ssh-port | SSH port for host |
| --ssh-key-path | SSH private key path |
| --ssh-key-pass | SSH passphrase of private key |
| --ssh-key-cert-path | SSH private key certificate path |
| --ssh-password | SSH login password |


HA(embedded etcd: >= 1.19.1-k3s1) mode need `--master-ips` at least 3 master nodes, e.g.
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip,master2-ip>
```

HA(external database) mode need `--master-ips` greater than 1 node, also need to specify `--datastore`, e.g.
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join K3s Nodes
```bash
autok3s join \
    --provider native \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --worker-ips <worker0-ip,worker1-ip>
```


Join master nodes to (embedded etcd: >= 1.19.1-k3s1) HA cluster e.g.
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip>
```

Join master nodes to (external database) HA cluster, also need to specify `--datastore`, e.g.
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Delete K3s Cluster
```bash
autok3s delete \
    --provider native \
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
```bash
autok3s ssh \
    --provider native \
    --name <cluster name>
```