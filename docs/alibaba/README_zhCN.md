# Alibaba Provider
使用阿里云SDK创建和管理主机，然后使用SSH将K3s群集安装到远程主机。 您也可以使用它将主机作为`masters/agents`加入K3s集群。

## 前置要求
以下样例使用阿里云 - `alibaba` ，如果使用子账号权限请参考 [RAMs](ram.md) 。
**安全组配置:**
请确保安全组至少开启了如下端口： 22(ssh默认使用),6443(kubectl默认使用),8999(如果开启ui需要使用)。

## 使用
使用命令 `autok3s <sub-command> --provider alibaba --help` 获取可用参数帮助。

### Create
创建实例并初始化一个K3s集群。
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
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

完整通用命令如下，可以在任何主机上执行。
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

高可用模式(嵌入式etcd: k3s版本 >= 1.19.1-k3s1) 要求 `--master` 至少为3。
```bash
autok3s ... \
    --master 3
```

高可用模式(外部数据库) 要求 `--master` 至少为1, 并且需要指定参数 `--datastore`。
```bash
autok3s ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join
为指定集群增加节点。
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
```bash
autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --worker 1
```

完整通用命令如下，可以在任何主机上执行。
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

为高可用集群(嵌入式etcd: k3s版本 >= 1.19.1-k3s1)模式新增节点。
```bash
autok3s ... \
    --master 2
```

为高可用集群(外部数据库)新增节点，需要指定参数`--datastore`。
```bash
autok3s ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Start
启动一个处于停止状态的K3s集群。
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
```bash
autok3s start \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

完整通用命令如下，可以在任何主机上执行。
```bash
autok3s start \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
```

### Stop
停止一个处于运行状态的K3s集群。
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

完整通用命令如下，可以在任何主机上执行。
```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
```

### Delete
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

完整通用命令如下，可以在任何主机上执行。
```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
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
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
```bash
autok3s ssh \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

完整通用命令如下，可以在任何主机上执行。
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

## 进阶使用
Autok3集成了一些与当前provider有关的高级组件，例如 terway、ccm、ui。

### 启用阿里云Terway CNI插件
实例的类型决定了K3S集群可以分配给集群POD的EIP数量，更多详细信息请参见[这里](https://www.alibabacloud.com/help/zh/doc-detail/97467.htm)。

```bash
autok3s create \
    ... \
    --terway "eni"
```

### 启用阿里云CCM(Cloud Controller Manager)

```bash
autok3s create \
    ... \
    --cloud-controller-manager
```

### 启用UI组件
该参数会启用 [kubernetes/dashboard](https://github.com/kubernetes/dashboard) 图形界面。
访问Token等设置请参考 [此文档](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) 。

```bash
autok3s create \
    ... \
    --ui
```
