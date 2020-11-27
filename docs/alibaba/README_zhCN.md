# Alibaba Provider
使用阿里云SDK创建和管理主机，然后使用SSH将K3s群集安装到远程主机。 您也可以使用它将主机作为`masters/agents`加入K3s集群。

## 前置要求
以下样例使用阿里云 - `alibaba` ，如果使用子账号权限请参考 [RAMs](ram.md) 。
**安全组配置:**
请确保安全组至少开启了如下端口

Protocol |  Port  | Source | Description
---|---|---|---|
TCP | 22 | all nodes | ssh 连接使用
TCP | 6443 | k3s agent nodes | kubernetes API使用
TCP | 10250 | k3s server and agent | kubelet 使用
TCP | 8999 | k3s dashboard | (可选)仅开启dashboard ui使用
UDP | 8472 | k3s server and agent | (可选)仅Flannel VXLAN使用
TCP | 2379, 2380 | k3s server nodes | (可选)etcd使用（如果使用外部数据库可忽略此项）

通常所有出站流量都被允许。

## 使用

### 快速启动命令

使用以下命令可以快速创建阿里云实例并初始化一个K3s集群。
```bash
export ECS_ACCESS_KEY_ID='<Your access key ID>'
export ECS_ACCESS_KEY_SECRET='<Your secret access key>'

autok3s create -p alibaba --name myk3s --master 1 --worker 1
```

或者

```
autok3s create -p alibaba --access-key <access-key> --access-secret <access-secret> --name myk3s --master 1 --worker 1
```

### 参数说明

使用命令 `autok3s <sub-command> --provider alibaba --help` 获取可用参数帮助。

一些参数可以通过CLI传入，也可以通过环境变量设置，以下为CLI参数及环境变量对照表：

参数 | 环境变量 | 描述 | 是否必填 | 默认值
---|---|---|---|---|
--access-key | ECS_ACCESS_KEY_ID | 访问阿里云API的access key | 是 |
--access-secret | ECS_ACCESS_KEY_SECRET | 访问阿里云API的secret key | 是 |
--name | | k3s集群名称 | 是 |
--region | ECS_REGION | 阿里云ECS region | 否 | cn-hangzhou
--zone | ECS_ZONE | 阿里云ECS region下的可用区 | 否 | cn-hangzhou-e
--key-pair | ECS_SSH_KEYPAIR | 阿里云ECS ssh key-pair | 否 |
--image | ECS_IMAGE_ID | ECS实例使用的操作系统镜像ID | 否 | ubuntu_18_04_x64_20G_alibase_20200618.vhd
--type | ECS_INSTANCE_TYPE | ECS实例规格 | 否 | ecs.c6.large
--v-switch | ECS_VSWITCH_ID | ECS使用交换机ID | 否 | autok3s-aliyun-vswitch
--disk-category | ECS_DISK_CATEGORY | ECS实例使用数据盘类型，`cloud_efficiency` 或 `cloud_ssd` | 否 | cloud_ssd
--disk-size | ECS_SYSTEM_DISK_SIZE | ECS实例系统盘大小 | 否 | 40GB
--security-group | ECS_SECURITY_GROUP | ECS实例使用的安全组 | 否 | autok3s
--cloud-controller-manager | | 是否开启cloud controller manager | 否 | false
--master-extra-args | | k3s master节点自定义配置参数 | 否 |
--worker-extra-args | | k3s worker节点自定义配置参数 | 否 |
--registries | | 私有镜像仓库地址 | 否 |
--datastore | | k3s集群使用的数据源（HA模式使用外部数据库时需要）| 否 |
--token | | k3s master token | 否 |
--master | | master节点数量 | 是 | 0
--worker | | worker节点数量 | 是 | 0
--repo | | helm 仓库地址 | 否 |
--terway | | 是否使用terway网络插件 | 否 | false
--terway-max-pool-size | | K3S集群可以分配给集群POD的EIP数量，如果`--terway`参数设置为`true`可以设置此项 | 否 | 5
--ui | | 是否部署dashboard ui | 否 | false

### Create
创建一个k3s集群。

```bash
autok3s create \
    --provider alibaba \
    --name <cluster name> \
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

```bash
autok3s join \
    --provider alibaba \
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
启动一个处于停止状态的k3s集群。

```bash
autok3s start \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

### Stop
停止一个处于运行状态的k3s集群。

```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

### Delete
删除一个k3s集群。

```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
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
SSH连接到集群中的某个主机。

```bash
autok3s ssh \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
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
