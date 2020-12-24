# Alibaba Provider
在阿里云ECS中创建对应VM实例，通过所创建的实例初始化k3s集群，或将一个或多个VM实例作为k3s节点加入到k3s集群中。

## 前置要求
为了确保ECS实例被正确创建及访问，请检查并设置以下内容。

### 设置环境变量
为运行`autok3s`命令的主机设置以下环境变量:

```bash
export ECS_ACCESS_KEY_ID='<access-key>'
export ECS_ACCESS_KEY_SECRET='<secret-access>'
```

### 设置 RAM
关于RAM的描述，请参考[这里](https://www.alibabacloud.com/help/zh/doc-detail/54235.htm).

需要以下权限来访问阿里云，因此需要确保为ECS实例创建以下RAM规则。

```json
{
  "Version": "1",
  "Statement": [
    {
      "Action": [
        "ecs:Describe*",
        "ecs:AttachDisk",
        "ecs:CreateDisk",
        "ecs:CreateSnapshot",
        "ecs:CreateRouteEntry",
        "ecs:DeleteDisk",
        "ecs:DeleteSnapshot",
        "ecs:DeleteRouteEntry",
        "ecs:DetachDisk",
        "ecs:ModifyAutoSnapshotPolicyEx",
        "ecs:ModifyDiskAttribute",
        "ecs:CreateNetworkInterface",
        "ecs:AttachNetworkInterface",
        "ecs:DetachNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:CreateNetworkInterface",
        "ecs:AttachNetworkInterface",
        "ecs:DetachNetworkInterface",
        "ecs:DeleteNetworkInterface",
        "ecs:AssignPrivateIpAddresses",
        "ecs:UnassignPrivateIpAddresses",
        "ecs:DeleteInstances",
        "ecs:RunInstances",
        "ecs:ListTagResources",
        "ecs:StartInstances",
        "ecs:StopInstances"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "cr:Get*",
        "cr:List*",
        "cr:PullRepository"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "slb:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "cms:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "vpc:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "log:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "nas:*"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    }
  ]
}
```

### 设置安全组
ECS实例需要应用以下安全组规则:

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

## 使用方式
更多参数请运行`autok3s <sub-command> --provider alibaba --help`命令。

### 快速启动
创建并启动一个k3s集群，这里集群为myk3s。

```bash
autok3s -d create -p alibaba --name myk3s --master 1 --worker 1
```

### 创建高可用K3s集群
高可用模式(嵌入式etcd: k3s版本 >= 1.19.1-k3s1) 要求 `--master` 至少为3。

```bash
autok3s -d create -p alibaba --name myk3s --master 3
```

高可用模式(外部数据库) 要求 `--master` 至少为1, 并且需要指定参数 `--datastore`。

```bash
autok3s -d create -p alibaba --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### 添加K3s节点
请指定你要添加K3s master/agent节点的集群, 这里为myk3s集群添加节点。

```bash
autok3s -d join --provider alibaba --name myk3s --worker 1
```

为高可用集群(嵌入式etcd: k3s版本 >= 1.19.1-k3s1)模式新增节点。

```bash
autok3s -d join --provider alibaba --name myk3s --master 2
```

为高可用集群(外部数据库)新增节点，需要指定参数`--datastore`。

```bash
autok3s -d join --provider alibaba --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### 启动K3s集群
启动一个处于停止状态的k3s集群，这里启动的集群为myk3s。

```bash
autok3s -d start --provider alibaba --name myk3s
```

### 停止K3s集群
停止一个处于运行状态的k3s集群，这里停止的集群为myk3s。

```bash
autok3s -d stop --provider alibaba --name myk3s
```

### 删除K3s集群
删除一个k3s集群，这里删除的集群为myk3s。

```bash
autok3s -d delete --provider alibaba --name myk3s
```

### 查看集群列表
显示当前主机上管理的所有K3s集群列表。

```bash
autok3s list
```

> 注意：使用公有云Provider创建的集群名称会使用`<clusterName>.<region>`显示。

```bash
       NAME         REGION     PROVIDER  STATUS   MASTERS  WORKERS    VERSION
myk3s.cn-hangzhou  cn-hangzhou  alibaba   Running  2        2        v1.19.5+k3s2
myk3s.ap-nanjing   ap-nanjing   tencent   Running  2        1        v1.19.5+k3s2
```

### 查看集群详细信息
显示具体的k3s信息，包括实例状态、主机ip、集群版本等信息。

```bash
autok3s describe cluster <clusterName>
```
> 注意：这里`<clusterName>`需要按照list显示的格式输入，例如`autok3s describe cluster myk3s.cn-hangzhou`

```bash
Name: myk3s.cn-hangzhou
Provider: alibaba
Region: cn-hangzhou
Zone: cn-hangzhou-i
Master: 2
Worker: 2
Status: Running
Version: v1.19.5+k3s2
Nodes:
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxx
    roles: etcd,master
    status: Ready
    hostname: xxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxxx
    roles: <none>
    status: Ready
    hostname: xxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxxxxx
    roles: etcd,master
    status: Ready
    hostname: xxxxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
  - internal-ip: x.x.x.x
    external-ip: x.x.x.x
    instance-status: Running
    instance-id: xxxxxxx
    roles: <none>
    status: Ready
    hostname: xxxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.19.5+k3s2
```

### Kubectl
集群创建完成后, `autok3s` 会自动合并 `kubeconfig` 文件。

```bash
autok3s kubectl config use-context myk3s.cn-hangzhou
autok3s kubectl <sub-commands> <flags>
```

在多个群集的场景下，可以通过切换上下文来完成对不同群集的访问。

```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

### SSH
SSH连接到集群中的某个主机，这里选择的集群为myk3s。

```bash
autok3s ssh --provider alibaba --name myk3s
```

## 进阶使用
我们集成了一些与当前provider有关的高级组件，例如 terway、ccm、ui。

### Setup Private Registry
下面是将本地的`/etc/autok3s/registries.yaml`启用TLS的`registry`配置文件，应用到通过`autok3s`命令应创建的k3s集群中。

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

在运行`autok3s create`或`autok3s join`时，通过传递`--registry /etc/autok3s/registries.yaml`参数使其生效，例如：

```bash
autok3s -d create \
    --provider alibaba \
    --name myk3s \
    --master 1 \
    --worker 1 \
    --registry /etc/autok3s/registries.yaml
```

### 启用阿里云Terway CNI插件
实例的类型决定了K3S集群可以分配给集群POD的EIP数量，更多详细信息请参见[这里](https://www.alibabacloud.com/help/zh/doc-detail/97467.htm)。

```bash
autok3s -d create \
    ... \
    --terway "eni"
```

### 启用阿里云CCM(Cloud Controller Manager)
```bash
autok3s -d create \
    ... \
    --cloud-controller-manager
```

### 启用UI组件
该参数会启用 [kubernetes/dashboard](https://github.com/kubernetes/dashboard) 图形界面。
访问Token等设置请参考 [此文档](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) 。

```bash
autok3s -d create \
    ... \
    --ui
```
