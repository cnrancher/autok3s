# Native Provider
在已经准备好的VM上，使用SSH来初始化k3s集群或为指定集群加入k3s节点。

## 前置要求
提供一个运行主流操作系统(如Ubuntu, Debian, Raspbian等)的新VM，并为它们注册或设置`SSH密钥/密码`。

### 设置安全组
VM实例需要应用以下安全组规则:

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
更多参数请运行`autok3s <sub-command> --provider native --help`命令。

### 快速启动
以下命令将创建一个k3s集群，这里集群为myk3s。

```bash
autok3s -d create \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master-ip-1,master-ip-2> \
    --worker-ips <worker-ip-1,worker-ip-2>
```

### 创建高可用K3s集群
高可用模式(嵌入式etcd: k3s版本 >= 1.19.1-k3s1) 要求 `--master` 至少为3。

```bash
autok3s -d create \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master-ip-1,master-ip-2,master-ip-3>
```

高可用模式(外部数据库) 要求 `--master` 至少为1, 并且需要指定参数 `--datastore`。

```bash
autok3s -d create \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master-ip-1,master-ip-2> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### 添加K3s节点
请指定你要添加K3s master/agent节点的集群, 这里为myk3s集群添加节点。

```bash
autok3s -d join \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --ip <existing-k3s-server-public-ip> \
    --worker-ips <worker-ip-2,worker-ip-3>
```

为高可用集群(嵌入式etcd: k3s版本 >= 1.19.1-k3s1)模式新增节点。

```bash
autok3s -d join \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --ip <existing-k3s-server-public-ip> \
    --master-ips <master-ip-2,master-ip-3>
```

为高可用集群(外部数据库)新增节点，需要指定参数`--datastore`。

```bash
autok3s -d join \
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --ip <existing-k3s-server-public-ip> \
    --master-ips <master-ip-2,master-ip-3> \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Kubectl
集群创建完成后, `autok3s` 会自动合并 `kubeconfig` 文件。

```bash
autok3s kubectl config use-context myk3s
autok3s kubectl <sub-commands> <flags>
```

在多个群集的场景下，可以通过切换上下文来完成对不同群集的访问。

```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

## 进阶使用
我们集成了一些与当前provider有关的高级组件，例如 ccm、ui。

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
    --provider native \
    --name myk3s \
    --ssh-key-path <ssh-key-path> \
    --master-ips <master-ip-1> \
    --worker-ips <worker-ip-1,worker-ip-2> \
    --registry /etc/autok3s/registries.yaml
```

### 启用UI组件
该参数会启用 [kubernetes/dashboard](https://github.com/kubernetes/dashboard) 图形界面。
访问Token等设置请参考 [此文档](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) 。

```bash
autok3s -d create \
    ... \
    --ui
```
