# Tencent Provider
使用腾讯云SDK创建和管理主机，然后使用SSH将K3s群集安装到远程主机。 您也可以使用它将主机作为`masters/agents`加入K3s集群。

## 前置要求
以下样例使用腾讯云 - `tencent`, 如果使用子账号权限请参考 [RAMs](../tencent/ram.md)。

## 使用
使用命令 `autok3s <sub-command> --provider tencent --help` 获取可用参数帮助。
**安全组配置:**
请确保安全组至少开启了如下端口： 22(ssh默认使用),6443(kubectl默认使用),8999(如果开启ui需要使用)。

**启用CCM**
启用CCM时，如果自定义群集的CIDR，则可能还需要创建路由表，以便POD可以通过VPC正常通信。
您可以从腾讯云控制台手动创建路由表，也可以通过[route-ctl](https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl)创建路由表。

### Create
创建实例并初始化一个K3s集群。
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
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

完整通用命令如下，可以在任何主机上执行。
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

启用EIP使用`--eip`。
```bash
autok3s ... \
    --eip
```

### Join
为指定集群增加节点。
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
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

完整通用命令如下，可以在任何主机上执行。
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
    --provider tencent \
    --region <region> \
    --name <cluster name>
```

完整通用命令如下，可以在任何主机上执行。
```bash
autok3s start \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
```

### Stop
停止一个处于运行状态的K3s集群。
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
```bash
autok3s stop \
    --provider tencent \
    --region <region> \
    --name <cluster name>
```

完整通用命令如下，可以在任何主机上执行。
```bash
autok3s stop \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
```

### Delete
如果在文件 `$HOME/.autok3s/config.yaml` 中已经有访问信息则可以使用以下简化命令。
```bash
autok3s delete \
    --provider tencent \
    --region <region> \
    --name <cluster name>
```

完整通用命令如下，可以在任何主机上执行。
```bash
autok3s delete \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --secret-id <secret-id> \
    --secret-key <secret-key>
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
    --provider tencent \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --ssh-user <ssh-user>
```

完整通用命令如下，可以在任何主机上执行。
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

## 进阶使用
Autok3集成了一些与当前provider有关的高级组件，例如 ccm、ui。

### 启用腾讯云CCM(Cloud Controller Manager)

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