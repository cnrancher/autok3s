<div align="center">
  <h1>AutoK3s</h1>
  <p>
    <img alt="Build Status" src="http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg">
    <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/cnrancher/autok3s">
    <img alt="GitHub release (latest by date)" src="https://img.shields.io/github/v/release/cnrancher/autok3s?color=default&label=release&logo=github">
    <img alt="GitHub release (latest by date including pre-releases)" src="https://img.shields.io/github/v/release/cnrancher/autok3s?include_prereleases&label=pre-release&logo=github">
  </p>
  <span>简体中文 / </span> <a href="../../../README.md">English</a>
</div>

<hr />

## 什么是 AutoK3s

[K3s](https://github.com/k3s-io/k3s) 是经过完全认证的 Kubernetes 产品, 在某些情况下可以替代完整的 K8s。

AutoK3s 是用于简化 K3s 集群管理的轻量级工具，您可以使用 AutoK3s 在任何地方运行 K3s 服务。

<!-- toc -->

- [关键特性](#关键特性)
- [云提供商](#云提供商)
- [快速体验](#快速体验)
- [使用指南](#使用指南)
- [演示视频](#演示视频)
- [开发者指南](#开发者指南)

<!-- /toc -->

## 关键特性

- 通过 API、CLI 和 UI 等方式快速创建 K3s。
- 云提供商集成（简化[CCM](https://kubernetes.io/docs/concepts/architecture/cloud-controller)设置）。
- 灵活安装选项，例如 K3s 集群 HA 和数据存储（内置 etcd、RDS、SQLite 等）。
- 低成本（尝试云中的竞价实例）。
- 通过 UI 简化操作。
- 多云之间弹性迁移，借助诸如[backup-restore-operator](https://github.com/rancher/backup-restore-operator)这样的工具进行弹性迁移。

## 云提供商

AutoK3s 可以支持以下云厂商，我们会根据社区反馈添加更多支持：

- [alibaba](alibaba/README.md) - 在阿里云的 ECS 中初始化 K3s 集群
- [tencent](tencent/README.md) - 在腾讯云 CVM 中初始化 K3s 集群
- [native](native/README.md) - 在任意类型 VM 实例中初始化 K3s 集群
- [aws](aws/README.md) - 在亚马逊 EC2 中初始化 K3s 集群

## 快速体验

通过 docker 运行:
```bash
# The commands will start autok3s daemon with an interactionable UI.

docker run -itd --restart=unless-stopped -p 8080:8080 cnrancher/autok3s:v0.4.0 serve --bind-address=0.0.0.0
```

通过 CLI 运行:

```bash
# 在 MacOS 或者 Linux 系统环境使用以下脚本安装 AutoK3s，Windows 用户请前往 Releases 页面下载对应的可执行程序。

curl -sS http://rancher-mirror.cnrancher.com/autok3s/install.sh  | INSTALL_AUTOK3S_MIRROR=cn sh

# 在亚马逊 EC2 上快速创建和启动一个 K3s 集群.

export ECS_ACCESS_KEY_ID='<Your access key ID>'
export ECS_ACCESS_KEY_SECRET='<Your secret access key>'

autok3s -d create -p aws --name myk3s --master 1 --worker 1
```

## 使用指南

AutoK3s 有两种运行模式：
- Local Mode： 在 Local Mode 模式下，您可以使用 CLI 或本地 UI 运行 AutoK3s。
- [开发中] Rancher Mode： 在这种模式下，您可以将 AutoK3s 集成到 Rancher 中，它将作为 Rancher 的扩展插件，使您可以构建一套托管 K3s 服务，通过 AutoK3s 创建的 K3s 集群可以自动导入 Rancher，并充分利用 Rancher 的 Kubernetes 管理功能。

## 演示视频

在以下演示中，我们将在 1 分钟左右的时间内把 K3s 安装到 AWS EC2 云主机上。

观看演示：

![](../../../docs/assets/autok3s-demo-min.gif)

## 开发者指南

使用 `Makefile` 管理项目的编译、测试与打包。
项目支持使用 `dapper`，`dapper`安装步骤请参考[dapper](https://github.com/rancher/dapper)。

- 依赖： `GO111MODULE=on go mod vendor`
- 编译： `BY=dapper make autok3s`
- 测试： `BY=dapper make autok3s unit`
- 打包： `BY=dapper make autok3s package only`
