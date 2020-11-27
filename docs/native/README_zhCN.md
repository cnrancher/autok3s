# Native Provider
它不集成Cloud SDK，而仅使用SSH来安装或加入k3s集群。

## 前置要求
以下演示使用了 `native` 提供程序，因此您需要配置一个新的VM，该VM运行兼容的操作系统，例如Ubuntu，CentOS等。
向新的VM或主机注册或设置 `SSH密钥/密码`。
**防火墙配置:**
请确保防火墙至少放行了如下端口：

Protocol |  Port  | Source | Description
---|---|---|---|
TCP | 22 | all nodes | ssh 连接使用
TCP | 6443 | k3s agent nodes | kubernetes API使用
TCP | 10250 | k3s server and agent | kubelet 使用
TCP | 8999 | k3s dashboard | (可选)仅开启dashboard ui使用
UDP | 8472 | k3s server and agent | (可选)仅Flannel VXLAN使用
TCP | 2379, 2380 | k3s server nodes | (可选)etcd使用(如果使用外部数据库可忽略此项)

通常所有出站流量都被允许。

## 使用
使用命令 `autok3s <sub-command> --provider native --help` 获取可用参数帮助。

### Create
创建集群。
```bash
autok3s create \
    --provider native \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master0-ip>
    --worker-ips <worker0-ip,worker1-ip>
```

高可用模式(嵌入式etcd: k3s版本 >= 1.19.1-k3s1) 要求 `--master` 至少为3。
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip,master2-ip>
```

高可用模式(外部数据库) 要求 `--master` 至少为1, 并且需要指定参数 `--datastore`。
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join
为集群新增节点。
```bash
autok3s join \
    --provider native \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --worker-ips <worker0-ip,worker1-ip>
```

为高可用集群(嵌入式etcd: k3s版本 >= 1.19.1-k3s1)模式新增节点。
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip>
```

为高可用集群(外部数据库)新增节点，需要指定参数`--datastore`。
```bash
autok3s ... \
    --master-ips <master0-ip,master1-ip> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Delete
删除集群。
```bash
autok3s delete \
    --provider native \
    --name <cluster name>
```

### List
显示当前主机上管理的所有K3s集群列表。
```bash
autok3s list
```

### Kubectl
集群创建完成后, `autok3s` 会自动合并 `kubeconfig` 文件。
```bash
autok3s kubectl <sub-commands> <flags>
```

在多个群集的场景下，可以通过切换上下文来完成对不同群集的访问。
```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

### SSH
连接到集群的特定节点。
```bash
autok3s ssh \
    --provider native \
    --name <cluster name>
```
## 进阶使用
Autok3集成了一些与当前provider有关的高级组件，例如 ccm、ui。

### 启用UI组件
该参数会启用 [kubernetes/dashboard](https://github.com/kubernetes/dashboard) 图形界面。
访问Token等设置请参考 [此文档](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) 。

```bash
autok3s create \
    ... \
    --ui
```
