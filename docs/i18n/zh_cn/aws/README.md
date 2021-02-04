# AWS Provider
在亚马逊创建对应的EC2实例，通过所创建的实例初始化k3s集群，或将一个或多个EC2实例作为k3s节点加入到k3s集群中。

## 前置要求
为了确保EC2实例被正确创建及访问，请检查并设置以下内容。

### 设置环境变量
为运行`autok3s`命令的主机设置以下环境变量:

```bash
export AWS_ACCESS_KEY_ID='<access-key>'
export AWS_SECRET_ACCESS_KEY='<secret-key>'
```

### 设置 IAM
关于IAM的描述，请参考[这里](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html?icmpid=docs_iam_console).

您的账号需要创建EC2及相关资源的权限，因此需要确保具有以下资源的权限。
您可以参考一下权限。
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:AuthorizeSecurityGroupIngress",
                "ec2:Describe*",
                "ec2:ImportKeyPair",
                "ec2:CreateKeyPair",
                "ec2:CreateSecurityGroup",
                "ec2:CreateTags",
                "ec2:DeleteKeyPair",
                "ec2:RunInstances",
                "ec2:RebootInstances",
                "ec2:TerminateInstances",
                "ec2:StartInstances",
                "ec2:StopInstances",
                "ec2:CreateInstanceProfile",
                "ec2:RevokeSecurityGroupIngress",
                "ec2:DeleteTags",
                "elasticloadbalancing:Describe*",
                "iam:Get*",
                "iam:List*",
                "iam:PassRole"
            ],
            "Resource": "*"
        }
    ]
}
```

### 设置安全组
EC2实例需要应用以下安全组规则:

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
更多参数请运行`autok3s <sub-command> --provider aws --help`命令。

### 快速启动
创建并启动一个k3s集群，这里集群为myk3s。

```bash
autok3s -d create -p aws --name myk3s --master 1 --worker 1
```

### 创建高可用K3s集群
高可用模式(嵌入式etcd: k3s版本 >= 1.19.1-k3s1)

```bash
autok3s -d create -p aws --name myk3s --master 3 --cluster
```

高可用模式(外部数据库) 要求 `--master` 至少为1, 并且需要指定参数 `--datastore`。

```bash
autok3s -d create -p aws --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### 添加K3s节点
请指定你要添加K3s master/agent节点的集群, 这里为myk3s集群添加节点。

```bash
autok3s -d join --provider aws --name myk3s --worker 1
```

为高可用集群(嵌入式etcd: k3s版本 >= 1.19.1-k3s1)模式新增节点。

```bash
autok3s -d join --provider aws --name myk3s --master 2
```

为高可用集群(外部数据库)新增节点，需要指定参数`--datastore`。

```bash
autok3s -d join --provider aws --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### 删除K3s集群
删除一个k3s集群，这里删除的集群为myk3s。

```bash
autok3s -d delete --provider aws --name myk3s
```

### 查看集群列表
显示当前主机上管理的所有K3s集群列表。

```bash
autok3s list
```

```bash
NAME         REGION      PROVIDER  STATUS   MASTERS  WORKERS    VERSION
myk3s    ap-southeast-2  aws   Running  1        0        v1.20.2+k3s1
```

### 查看集群详细信息
显示具体的k3s信息，包括实例状态、主机ip、集群版本等信息。

```bash
autok3s describe cluster <clusterName>
```
> 注意：如果使用不同的provider创建的集群名称相同，describe时会显示多个集群信息，可以使用`-p <provider> -r <region>`对provider及region进一步过滤。e.g. `autok3s describe cluster <clusterName> -p aws -r <region>`

```bash
Name: myk3s
Provider: aws
Region: ap-southeast-2
Zone: ap-southeast-2c
Master: 1
Worker: 0
Status: Running
Version: v1.20.2+k3s1
Nodes:
  - internal-ip: [x.x.x.x]
    external-ip: [x.x.x.x]
    instance-status: running
    instance-id: xxxxxxxx
    roles: control-plane,master
    status: Ready
    hostname: xxxxxxxx
    container-runtime: containerd://1.4.3-k3s1
    version: v1.20.2+k3s1
```

### Kubectl
集群创建完成后, `autok3s` 会自动合并 `kubeconfig` 文件。

```bash
autok3s kubectl config use-context myk3s.ap-southeast-2.aws
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
autok3s ssh --provider aws --name myk3s
```

## 进阶使用
我们集成了一些与当前provider有关的高级组件，例如ccm、ui。

### Setup Private Registry
在运行`autok3s create`或`autok3s join`时，通过传递`--registry /etc/autok3s/registries.yaml`以使用私有镜像仓库，例如：

```bash
autok3s -d create \
    --provider aws \
    --name myk3s \
    --master 1 \
    --worker 1 \
    --registry /etc/autok3s/registries.yaml
```

使用私有镜像仓库的配置请参考以下内容，如果您的私有镜像仓库需要TLS认证，`autok3s`会从本地读取相关的TLS文件并自动上传到远程服务器中完成配置，您只需要完善`registry.yaml`即可。
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

### 启用AWS Cloud Controller Manager

如果您要使用AWS CCM功能需要提前准备好两个IAM policies，以保证CCM功能的正常使用，具体内容请参考[这里](https://kubernetes.github.io/cloud-provider-aws/prerequisites.html)

```bash
autok3s -d create -p aws \
    ... \
    --cloud-controller-manager \
    --iam-instance-profile-control <iam policy for control plane> \
    --iam-instance-profile-worker <iam policy for node>
```

### 启用UI组件
该参数会启用 [kubernetes/dashboard](https://github.com/kubernetes/dashboard) 图形界面。
访问Token等设置请参考 [此文档](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) 。

```bash
autok3s -d create -p aws \
    ... \
    --ui
```
