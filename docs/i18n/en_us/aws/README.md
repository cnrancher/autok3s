# AWS Provider

English / [简体中文](https://github.com/cnrancher/autok3s/blob/master/docs/i18n/zh_cn/aws/README.md)

## Introduction

This article provides users with the instructions to create and launch a K3s cluster on an AWS EC2 instance, and to add nodes for an existing K3s cluster on AWS EC2 instance. In additon, this article provides guidance of advanced usages of running K3s on AWS EC2, such as setting up private registry, enabling AWS CCM, and enabling UI components.

## Prerequisites

To ensure that EC2 instances can be created and accessed successfully, please follow the instructions below.

### Setting up Environment

Configure the following environment variables for the host on which you are running `autok3s`.

```bash
export AWS_ACCESS_KEY_ID='<access-key>'
export AWS_SECRET_ACCESS_KEY='<secret-key>'
```

### Setting up IAM

Please refer [here](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html?icmpid=docs_iam_console) for more IAM settings.

Please make sure your account has permission to manage EC2 instance or other relative resources.

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

### Setting up Security Group

The EC2 instances need to apply the following **minimum** Security Group Rules:

<details>

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

</details>

## Creating a K3s cluster

Please use `autok3s create` command to create a cluster in your EC2 instance.

### Normal Cluster

The following command uses AWS as cloud provider, creates a K3s cluster named "myk3s", and assign it with 1 master node and 1 worker node:

```bash
autok3s -d create -p aws --name myk3s --master 1 --worker 1
```

### HA Cluster

Please use one of the following commands to create an HA cluster.

#### Embedded etcd

The following command uses AWS as cloud provider, creates an HA K3s cluster named "myk3s", and assigns it with 3 master nodes.

```bash
autok3s -d create -p aws --name myk3s --master 3 --cluster
```

#### External Database

The following requirements must be met before creating an HA K3s cluster with external database:

- The number of master nodes in this cluster must be greater or equal to 1.
- The external database information must be specified within `--datastore "PATH"` parameter.

In the example below, `--master 2` specifies the number of master nodes to be 2, `--datastore "PATH"` specifies the external database information. As a result, requirements listed above are met.

Run the command below and create an HA K3s cluster with external database:

```bash
autok3s -d create -p aws --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

## Join K3s Nodes

Please use `autok3s join` command to add one or more nodes for an existing K3s cluster.

### Normal Cluster

The command below shows how to add a worker node for an existing K3s cluster named "myk3s".

```bash
autok3s -d join --provider aws --name myk3s --worker 1
```

### HA Cluster

The commands to add one or more nodes for an existing HA K3s cluster varies based on the types of HA cluster. Please choose one of the following commands to run.

#### Embedded etcd

Run the command below, to add 2 master nodes for an Embedded etcd HA cluster(embedded etcd: >= 1.19.1-k3s1).

```bash
autok3s -d join --provider aws --name myk3s --master 2
```

#### External Database

Run the command below, to add 2 master nodes for an HA cluster with external database, you will need to fill in `--datastore "PATH"` as well.

```bash
autok3s -d join --provider aws --name myk3s --master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

## Delete K3s Cluster

This command will delete a k3s cluster named "myk3s".

```bash
autok3s -d delete --provider aws --name myk3s
```

## List K3s Clusters

This command will list the clusters that you have created on this machine.

```bash
autok3s list
```

```bash
NAME         REGION      PROVIDER  STATUS   MASTERS  WORKERS    VERSION
myk3s    ap-southeast-2  aws   Running  1        0        v1.20.2+k3s1
```

## Describe k3s cluster

This command will show detail information of specified cluster, such as instance status, node IP, kubelet version, etc.

```bash
autok3s describe cluster <clusterName>
```

> Note：There will be multiple results if using the same name to create with different providers, please use `-p <provider> -r <region>` to choose specified cluster. e.g. `autok3s describe cluster <clusterName> -p aws -r <region>`

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

## Access K3s Cluster

After the cluster is created, `autok3s` will automatically merge the `kubeconfig` so that you can access the cluster.

```bash
autok3s kubectl config use-context myk3s.ap-southeast-2.aws
autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.

```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

## SSH K3s Cluster's Node

Login to a specific k3s cluster node via ssh, e.g myk3s.

```bash
autok3s ssh --provider aws --name myk3s
```

## Other Usages

More usage details please running `autok3s <sub-command> --provider aws --help` commands.

## Advanced Usages

We integrate some advanced components such as private registries, AWS Cloud Controller Manager(CCM) and UI, related to the current provider.

### Setting up Private Registry

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

When running `autok3s create` or `autok3s join` command, it will take effect with the`--registry /etc/autok3s/registries.yaml` flag, e.g:

```bash
autok3s -d create \
    --provider aws \
    --name myk3s \
    --master 1 \
    --worker 1 \
    --registry /etc/autok3s/registries.yaml
```

### Enabling AWS Cloud Controller Manager(CCM)

Please check [this](https://kubernetes.github.io/cloud-provider-aws/prerequisites.html) to prepare IAM policies as prerequisites.

```bash
autok3s -d create -p aws \
    ... \
    --cloud-controller-manager \
    --iam-instance-profile-control <iam policy for control plane> \
    --iam-instance-profile-worker <iam policy for node>
```

### Enable UI Component

This flag will enable [kubernetes/dashboard](https://github.com/kubernetes/dashboard) UI component.
Please following this [docs](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) to create user token.

```bash
autok3s -d create -p aws \
    ... \
    --ui
```
