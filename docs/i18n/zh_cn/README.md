# autok3s

[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnrancher/autok3s)](https://goreportcard.com/report/github.com/cnrancher/autok3s)
![GitHub release](https://img.shields.io/github/v/release/cnrancher/autok3s.svg?color=blue)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?color=blue)](http://github.com/cnrancher/autok3s/pulls)

简体中文 / [English](../../../README.md)

## 什么是 AutoK3s

K3s 是经过完全认证的 Kubernetes 产品，在某些情况下可以替代完整的 K8s。使用 K3s 在公有云中创建集群的过程比较复杂，在各个公有云中创建 K3s 集群所需填写的参数也有差异。为了降低 K3s 的使用门槛，简化使用 K3s 的过程，我们开发了 AutoK3s 这一款辅助工具。

AutoK3s 是用于简化 K3s 集群管理的轻量级工具，您可以使用 Autok3s 在任何地方运行 K3s，Run K3s Everywhere。您可以使用 AutoK3s 快速创建并启动 K3s 集群，也可以使用它来为已有的 K3s 集群添加节点，不仅提升公有云的使用体验，同时还继承了 kubectl，提供了便捷的集群能力。目前 AutoK3s 支持的云服务提供商包括**阿里云、腾讯云和 AWS**，如果您使用的云服务提供商不属于以上三家，您可以使用`native`模式，在任意类型的虚拟机实例中初始化 k3s 集群。在后续的开发过程中，我们会根据社区反馈，为其他云服务提供商提供适配。

<!-- toc -->

- [关键特性](#关键特性)
- [常用命令](#常用命令)
- [常用参数](#常用参数)
- [支持的云提供商](#支持的云提供商)
- [快速体验](#快速体验)
- [演示视频](#演示视频)
- [开发者指南](#开发者指南)

<!-- /toc -->

## 关键特性

- 通过 API，CLI 和 UI 等方式快速创建 K3s。
- 云提供商集成（简化每个云的[CCM](https://kubernetes.io/docs/concepts/architecture/cloud-controller)设置）。
- 灵活的安装选项，例如 K3s 群集 HA 和数据存储（内置 etcd，RDS，SQLite 等）。
- 低成本（尝试每个云中的竞价实例）。
- 通过 UI 简化操作。
- 多云之间弹性迁移，借助诸如[backup-restore-operator](https://github.com/rancher/backup-restore-operator)这样的工具。

## 常用命令

- `autok3s create`：创建和启动 K3s 集群。
- `autok3s join`：为已有的 K3s 集群添加节点。

## 常用参数

AutoK3s 命令中常用的参数如下表所示：

| 参数               | 描述                                     |
| :----------------- | :--------------------------------------- |
| `-d`或`--debug`    | 开启 debug 模式。                        |
| `-p`或`--provider` | provider，即云服务提供商，详情参考下表。 |
| `-n`或`--name`     | 指定将要创建的集群的名称。               |
| `--master`         | 指定创建的 master 节点数量。             |
| `--worker`         | 指定创建的 worker 节点数量。             |

### 云服务提供商参数描述

| 参数                               | 描述                         |
| :--------------------------------- | :--------------------------- |
| `-p alibaba`或`--provider alibaba` | 指定阿里云作为云服务提供商。 |
| `-p tencent`或`--provider tencent` | 指定腾讯云作为云服务提供商。 |
| `-p aws`或`--provider aws`         | 指定 AWS 作为云服务提供商。  |

## 支持的云提供商

AutoK3s 可以支持以下云厂商，我们会根据社区反馈添加更多支持：

- [alibaba](alibaba/README.md) - 在阿里云的 ECS 中初始化 K3s 集群
- [tencent](tencent/README.md) - 在腾讯云 CVM 中初始化 K3s 集群
- [native](native/README.md) - 在任意类型 VM 实例中初始化 K3s 集群
- [aws](aws/README.md) - 在亚马逊 EC2 中初始化 K3s 集群

## 快速体验

AutoK3s 有两种运行模式：Local Mode 和 Rancher Mode。

### Local Mode

在 Local Mode 模式下，您可以使用 CLI 或本地 UI 运行 AutoK3s。

#### CLI

以下代码是创建 K3s 集群和添加节点的示例，请在运行之前检查[前提条件](alibaba/README.md)。

##### 创建 K3s 集群

`autok3s create` 命令的表达式可以概括为：

```bash
autok3s -d create -p <云服务提供商> --name <集群名称> --master <master节点数量> --worker <worker节点数量>
```

**示例**
这个命令使用了阿里云`alibaba`作为云提供商，在阿里云上创建了一个名为 “myk3s”的集群，并为该集群配置了 1 个 master 节点和 1 个 worker 节点：

```bash
export ECS_ACCESS_KEY_ID='<Your access key ID>'
export ECS_ACCESS_KEY_SECRET='<Your secret access key>'

autok3s -d create -p alibaba --name myk3s --master 1 --worker 1
```

##### 添加节点

`autok3s join`命令的表达式可以概括为：

```bash
autok3s -d join -p <云服务提供商> --name <集群名称> --master <master节点数量> --worker <worker节点数量>
```

其中，`-p <云服务提供商>`和`--name <集群名称>`为必填项，用于指定云服务提供商和添加的集群名称；`--master <master节点数量>`和`--worker <worker节点数量>`为选填项，用于指定添加的节点数量，如果您只需要单独添加 master 或 worker 节点，则可以不填写另一个类型节点的参数，也不需要指定这个类型的节点数量。

**示例**
以下代码是为已有的 K3s 集群添加 K3s 节点的示例。名为“myk3s”的集群是已经运行在阿里云上 的 K3s 集群。这个命令使用了阿里云`alibaba`作为云提供商，为“myk3s”集群添加了 1 个 worker 节点。

```bash
autok3s -d join --provider alibaba --name myk3s --worker 1
```

#### UI

如果要启用本地 UI，请运行 `autok3s serve`，如下图所示。

![autok3s-local-ui](../../assets/autok3s-local-ui.png)

### Rancher Mode

在这种模式下，您可以将 AutokK3s 放入[Rancher](https://github.com/rancher/rancher)。
它将作为 Rancher 的扩展，使您可以构建一套托管 K3s 服务。

AutoK3s 创建的 K3s 集群可以自动导入 Rancher，并充分利用 Rancher 的 Kubernetes 管理功能。

此模式正在开发中。

## 演示视频

在以下演示中，我们将在 1 分钟左右的时间内把 K3s 安装到 Alibaba ECS 云主机上。

观看演示：

[![asciicast](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg.svg)](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg)

## 开发者指南

使用 `Makefile` 管理项目的编译、测试与打包。
项目支持使用 `dapper`，`dapper`安装步骤请参考[dapper](https://github.com/rancher/dapper)。

- 依赖： `GO111MODULE=on go mod vendor`
- 编译： `BY=dapper make autok3s`
- 测试： `BY=dapper make autok3s unit`
- 打包： `BY=dapper make autok3s package only`
