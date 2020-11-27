# Tencent Provider
使用腾讯云SDK创建和管理主机，然后使用SSH将K3s群集安装到远程主机。 您也可以使用它将主机作为`masters/agents`加入K3s集群。

## 前置要求
以下样例使用腾讯云 - `tencent`, 如果使用子账号权限请参考 [RAMs](../tencent/ram.md)。

## 使用
使用命令 `autok3s <sub-command> --provider tencent --help` 获取可用参数帮助。
**安全组配置:**
请确保安全组至少开启了如下端口：

Protocol |  Port  | Source | Description
---|---|---|---|
TCP | 22 | all nodes | ssh 连接使用
TCP | 6443 | k3s agent nodes | kubernetes API使用
TCP | 10250 | k3s server and agent | kubelet 使用
TCP | 8999 | k3s dashboard | (可选)仅开启dashboard ui使用
UDP | 8472 | k3s server and agent | (可选)仅Flannel VXLAN使用
TCP | 2379, 2380 | k3s server nodes | (可选)etcd使用(如果使用外部数据库可忽略此项)

通常所有出站流量都被允许。

> 注意：腾讯云的安全组默认出站流量都被禁止，请根据自己的需要开启设置。

**启用CCM**
启用CCM时，需要手动创建路由表，以便POD可以通过VPC正常通信，具体请参考[这里](#启用腾讯云CCM(Cloud Controller Manager))

## 使用

使用以下命令可以快速创建腾讯云实例并初始化一个K3s集群。

```bash
export CVM_SECRET_ID='<Your secret id>'
export CVM_SECRET_KEY='<Your secret key>'

autok3s create -p tencent --name myk3s --master 1 --worker 1 --ssh-user ubuntu
```

或者

```
autok3s create -p tencent --secret-id <secret-id> --secret-key <secret-key> --name myk3s --master 1 --worker 1 --ssh-user ubuntu
```

### 参数说明

使用命令 `autok3s <sub-command> --provider tencent --help` 获取可用参数帮助。

一些参数可以通过CLI传入，也可以通过环境变量设置，以下为CLI参数及环境变量对照表：

参数 | 环境变量 | 描述 | 是否必填 | 默认值
---|---|---|---|---|
--secret-id | CVM_SECRET_ID | 访问腾讯云API的secret id | 是 |
--secret-key | CVM_SECRET_KEY | 访问腾讯云API的secret key | 是 |
--name | | k3s集群名称 | 是 |
--region | CVM_REGION | 腾讯云实例所在区域 | 否 | ap-guangzhou
--zone | CVM_ZONE | 腾讯云区域下的可用区 | 否 | ap-guangzhou-3
--key-pair | CVM_SSH_KEYPAIR | 腾讯云通过ssh连接实例的key-pair | 否 |
--image | | 腾讯云实例使用的操作系统镜像ID | 否 | img-pi0ii46r(Ubuntu Server 18.04.1 LTS x64)
--type | CVM_INSTANCE_TYPE | 腾讯云实例规格 | 否 | SA1.MEDIUM4
--vpc | CVM_VPC_ID | 腾讯云实例使用的私有网络 | 否 | autok3s-tencent-vpc
--subnet | CVM_SUBNET_ID | 腾讯云实例私有网络下的子网 | 否 | autok3s-tencent-subnet
--disk-category | CVM_DISK_CATEGORY | 腾讯云实例使用数据盘类型 | 否 | CLOUD_SSD
--disk-size | CVM_DISK_SIZE | 腾讯云实例系统盘大小 | 否 | 60GB
--security-group | CVM_SECURITY_GROUP | 腾讯云实例使用的安全组 | 否 | autok3s
--cloud-controller-manager | | 是否开启cloud controller manager | 否 | false
--master-extra-args | | k3s master节点自定义配置参数 | 否 |
--worker-extra-args | | k3s worker节点自定义配置参数 | 否 |
--registries | | 私有镜像仓库地址 | 否 |
--datastore | | k3s集群使用的数据源（HA模式使用外部数据库时需要）| 否 |
--token | | k3s master token | 否 |
--master | | master节点数量 | 是 | 0
--worker | | worker节点数量 | 是 | 0
--repo | | helm 仓库地址 | 否 |
--ui | | 是否部署dashboard ui | 否 | false
--router | | 启动CCM使用的路由表名称，如果开启了cloud controller manager，则必须传入此项 | 否 | 

### Create
创建实例并初始化一个K3s集群。

```bash
autok3s create \
    --provider tencent \
    --name <cluster name> \
    --ssh-user ubuntu \
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

```bash
autok3s join \
    --provider tencent \
    --region <region> \
    --name <cluster name> \
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
删除k3s集群。
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
SSH连接到集群中的某个主机。
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

如果启用CCM，您需要提前创建好集群路由表，以便POD可以通过VPC正常通信，并将路由表的名称通过`--router`参数传入。

autok3s默认使用的cluster cidr为`10.42.0.0/16`，您需要为该网段创建路由表。

您可以通过[route-ctl](https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl)创建

```bash
export QCloudSecretId=************************************
export QCloudSecretKey=********************************
export QCloudCcsAPIRegion=<your-region>

./route-ctl route-table create --route-table-cidr-block 10.42.0.0/16 --route-table-name <your-route-table-name> --vpc-id <your-vpc-id>
```

接下来将上面创建好的`<your-route-table-name>`作为`--router`参数。这里注意--vpc也要使用创建router的vpc id。
```bash
autok3s create \
    ... \
    --cloud-controller-manager --router <your-route-table-name> --vpc <your-vpc-id> --subnet <your-subnet-id>
```

在您删除集群后，集群路由不会**不会自动删除**，您可以使用[route-ctl](https://github.com/TencentCloud/tencentcloud-cloud-controller-manager/tree/master/route-ctl)手动删除。

### 启用UI组件
该参数会启用 [kubernetes/dashboard](https://github.com/kubernetes/dashboard) 图形界面。
访问Token等设置请参考 [此文档](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) 。

```bash
autok3s create \
    ... \
    --ui
```