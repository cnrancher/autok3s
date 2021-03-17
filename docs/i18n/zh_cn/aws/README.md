# AWS Provider

简体中文 / [English](https://github.com/cnrancher/autok3s/blob/master/docs/i18n/en_us/aws/README.md)

## 概述

本文介绍了如何在 AWS EC2 中创建和初始化 K3s 集群，以及为已有的 K3s 集群添加节点的操作步骤。除此之外，本文还提供了在 AWS EC2 上运行 AutoK3s 的进阶操作指导，如配置私有镜像仓库、、启用 AWS CCM 和启用 UI 组件。

## 前置要求

为了能够成功创建 EC2 实例，以及能够在创建完成后成功访问到该实例，请按照以下步骤设置环境变量、IAM 和安全组。

### 设置环境变量

运行以下命令，为运行`autok3s`命令的主机设置以下环境变量：

```bash
export AWS_ACCESS_KEY_ID='<access-key>'
export AWS_SECRET_ACCESS_KEY='<secret-key>'
```

### 设置 IAM

关于 IAM 的描述，请参考[AWS 官方文档](https://docs.aws.amazon.com/zh_cn/IAM/latest/UserGuide/id_roles.html?icmpid=docs_iam_console).

您的账号需要创建 EC2 及相关资源的权限，因此需要确保具有以下资源的权限：

<details>

```json
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

</details>

### 设置安全组

EC2 实例**至少**需要应用以下安全组规则：

<details>

```bash
Rule        Protocol    Port      Source             Description
InBound     TCP         22        ALL                SSH Connect Port
InBound     TCP         6443      K3s agent nodes    Kubernetes API
InBound     TCP         10250     K3s server & agent Kubelet
InBound     UDP         8472      K3s server & agent (Optional) Required only for Flannel VXLAN
InBound     TCP         2379,2380 K3s server nodes   (Optional) Required only for embedded ETCD
OutBound    ALL         ALL       ALL                Allow All
```

</details>

## 创建集群

请使用`autok3s create`命令在 AWS EC2 实例中创建集群。

### 创建普通集群

运行以下命令，在 AWS EC2 上创建并启动创建一个名为 “myk3s”的集群，并为该集群配置 1 个 master 节点和 1 个 worker 节点。

```bash
autok3s -d create -p aws --name myk3s --master 1 --worker 1
```

### 创建高可用 K3s 集群

创建高可用集群的命令分为两种，取决于您选择使用的是内置的 etcd 还是外部数据库。

#### 嵌入式 etcd（k3s 版本 >= 1.19.1-k3s1)

运行以下命令，在 AWS EC2 上创建并启动创建了一个名为“myk3s”的高可用 K3s 集群。

```bash
autok3s -d create -p aws --name myk3s --master 3 --cluster
```

#### 外部数据库

在高可用模式下使用外部数据库，需要满足两个条件：

- master 节点的数量不小于 1。
- 需要提供外部数据库的存储路径。

所以在以下的代码示例中，我们通过`--master 2`指定 master 节点数量为 2，满足 master 节点的数量不小于 1 这个条件；且通过`--datastore "PATH"`指定外部数据库的存储路径，提供外部数据库的存储路径。

运行以下命令，在 AWS EC2 上创建并启动创建了一个名为“myk3s”的高可用 K3s 集群：

```bash
autok3s -d create -p aws --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

## 添加 K3s 节点

请使用`autok3s join`命令为已有集群添加 K3s 节点。

### 普通集群

运行以下命令，为“myk3s”集群添加 1 个 worker 节点。

```bash
autok3s -d join --provider aws --name myk3s --worker 1
```

### 高可用 K3s 集群

添加 K3s 节点的命令分为两种，取决于您选择使用的是内置的 etcd 还是外部数据库。

#### 嵌入式 etcd

```bash
autok3s -d join --provider aws --name myk3s --master 2
```

#### 外部数据库

运行以下命令，为高可用集群（嵌入式 etcd: k3s 版本 >= 1.19.1-k3s1）“myk3s”集群添加 2 个 master 节点。

```bash
autok3s -d join --provider aws --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

## 删除 K3s 集群

删除一个 k3s 集群，这里删除的集群为 myk3s。

```bash
autok3s -d delete --provider aws --name myk3s
```

## 查看集群列表

显示当前主机上管理的所有 K3s 集群列表。

```bash
autok3s list
```

```bash
NAME         REGION      PROVIDER  STATUS   MASTERS  WORKERS    VERSION
myk3s    ap-southeast-2  aws   Running  1        0        v1.20.2+k3s1
```

## 查看集群详细信息

显示具体的 K3s 信息，包括实例状态、主机 ip、集群版本等信息。

```bash
autok3s describe -n <clusterName> -p aws
```

> 注意：如果使用不同的 provider 创建的集群名称相同，describe 时会显示多个集群信息，可以使用`-p <provider>`对 provider 进一步过滤。例如：`autok3s describe -n myk3s -p aws`。

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

## Kubectl

群创建完成后, `autok3s` 会自动合并 `kubeconfig` 文件。

```bash
autok3s kubectl config use-context myk3s.ap-southeast-2.aws
autok3s kubectl <sub-commands> <flags>
```

在多个集群的场景下，可以通过切换上下文来完成对不同集群的访问。

```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

## SSH

SSH 连接到集群中的某个主机，这里选择的集群为 myk3s。

```bash
autok3s ssh --provider aws --name myk3s
```

## 其他功能

更多参数请运行`autok3s <sub-command> --provider aws --help`命令。

## 进阶使用

AutoK3s 集成了一些与当前 provider 有关的高级组件，例如私有镜像仓库、CCM 和 UI。

### 配置私有镜像仓库

在运行`autok3s create`或`autok3s join`时，通过传递`--registry /etc/autok3s/registries.yaml`以使用私有镜像仓库，例如：

```bash
autok3s -d create \
    --provider aws \
    --name myk3s \
    --master 1 \
    --worker 1 \
    --registry /etc/autok3s/registries.yaml
```

使用私有镜像仓库的配置请参考以下内容，如果您的私有镜像仓库需要 TLS 认证，`autok3s`会从本地读取相关的 TLS 文件并自动上传到远程服务器中完成配置，您只需要完善`registry.yaml`即可。

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

### 启用 AWS CCM(Cloud Controller Manager)

如果您要使用 AWS CCM 功能需要提前准备好两个 IAM policies，以保证 CCM 功能的正常使用，具体内容请参考[这里](https://github.com/kubernetes/cloud-provider-aws/blob/master/docs/prerequisites.md)

```bash
autok3s -d create -p aws \
    ... \
    --cloud-controller-manager \
    --iam-instance-profile-control <iam policy for control plane> \
    --iam-instance-profile-worker <iam policy for node>
```

### 启用 UI 组件

该参数会启用 [kubernetes/dashboard](https://github.com/kubernetes/dashboard) 图形界面。
访问 Token 等设置请参考 [此文档](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) 。

```bash
autok3s -d create -p aws \
    ... \
    --ui
```
