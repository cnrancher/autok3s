# autok3s

[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnrancher/autok3s)](https://goreportcard.com/report/github.com/cnrancher/autok3s)
![GitHub release](https://img.shields.io/github/v/release/cnrancher/autok3s.svg?color=blue)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?color=blue)](http://github.com/cnrancher/autok3s/pulls)

简体中文 / [English](../../../README.md)

## 什么是 autok3s

使用 K3s 在公有云中创建集群的过程比较复杂，在各个公有云中创建 K3s 集群所需填写的参数也有差异。为了降低 K3s 的使用门槛，简化使用 K3s 的过程，我们开发了 autok3s 这一款辅助工具。您可以使用 autok3s 快速创建并启动 k3s 集群，同时可以使用它来为已有的 k3s 集群添加节点，不仅提升公有云的使用体验，同时还继承了 kubectl，提供了便捷的集群能力。目前 autok3s 支持的云服务提供商包括**阿里云、腾讯云和 AWS**，如果您使用的云服务提供商不属于以上三家，您可以使用`native`模式，在任意类型的虚拟机实例中初始化 k3s 集群。在后续的开发过程中，我们会考虑为其他云服务提供商提供适配。

<!-- toc -->

- [关键特性](#关键特性)
- [常用命令](#常用命令)
- [常用参数](#常用参数)
- [支持的云提供商](#支持的云提供商)
- [快速体验](#快速体验)
- [演示视频](#演示视频)
- [开发者指南](#开发者指南)
- [开源协议](#开源协议)

<!-- /toc -->

## 关键特性

- 使用 `autok3s create` 命令，在多个公有云提供商中快速启动 Kubernetes (k3s) 集群
- 使用 `autok3s join` 命令，添加节点至已存在的 Kubernetes (k3s) 集群
- 自动为创建的 Kubernetes (k3s) 集群生成可供访问的 `kubeconfig` 文件
- 集成 `kubectl` 以提供访问集群的能力
- 支持创建 Kubernetes k3s HA 集群
- 使用`--registry`以支持配置`containerd`私有镜像仓库
- 集成扩展参数 `--cloud-controller-manager` 以开启 Kubernetes Cloud-Controller-Manager 组件
- 集成扩展参数 `--ui` 以开启 Kubernetes Dashboard UI 组件
- 集成扩展参数  例如 `--terway 'eni'` 以开启公有云 CNI 网络插件

## 常用命令

- `autok3s create`：创建和启动 k3s 集群
- `autok3s join`：为已有的 k3s 集群添加节点

## 常用参数

autok3s 命令中常用的参数如下表所示：

| 参数                         | 描述                                            |
| :--------------------------- | :---------------------------------------------- |
| `-d`                         | 开启 debug 模式。                               |
| `-p`                         | provider，即云服务提供商，详情参考下表。        |
| `--name`                     | 指定将要创建的集群的名称。                      |
| `--master`                   | 指定创建的 master 节点数量 。                   |
| `--worker`                   | 指定创建的 worker 节点数量 。                   |
| `--registry`                 | 配置`containerd`私有镜像仓库。                  |
| `--cloud-controller-manager` | 开启 Kubernetes Cloud-Controller-Manager 组件。 |
| `--ui`                       | 开启 Kubernetes Dashboard UI 组件。             |
| `--terway 'eni'`             | 开启公有云 CNI 网络插件（仅适用于阿里云）。     |

### 云服务提供商参数描述

| 参数         | 描述                       |
| :----------- | :------------------------- |
| `-p alibaba` | 指定阿里云作为云服务提供商 |
| `-p tencent` | 指定腾讯云作为云服务提供商 |
| `-p aws`     | 指定 AWS 作为云服务提供商  |

## 支持的云提供商

有关更多用法的详细信息，请参见下面的链接：

<<<<<<< HEAD
- [阿里云](alibaba/README.md) - 在阿里云的 ECS 中初始化 Kubernetes (k3s) 集群。
- [腾讯云](tencent/README.md) - 在腾讯云 CVM 中初始化 Kubernetes (k3s) 集群。
- [AWS](aws/README.md) - 在 AWS EC2 中初始化 Kubernetes (k3s) 集群。
- [native](native/README.md) - 在任意类型 VM 实例中初始化 Kubernetes (k3s) 集群。
=======
- [alibaba](alibaba/README.md) - 在阿里云的 ECS 中初始化 Kubernetes (k3s) 集群
- [tencent](tencent/README.md) - 在腾讯云 CVM 中初始化 Kubernetes (k3s) 集群
- [native](native/README.md) - 在任意类型 VM 实例中初始化 Kubernetes (k3s) 集群
- [aws](aws/README.md) - 在亚马逊 EC2 中初始化 Kubernetes (k3s) 集群
>>>>>>> 06d3c89... feat(autok3s): support hosted UI and API

## 示例

`autok3s create` 和 `autok3s join`这两个常用的 autok3s 命令通常需要配合多个参数使用，以下是两个命令的使用示例。您可以调整的参数包括：云供应商、节点名称、master 节点数量和 worker 节点数量。

### 创建 k3s 集群

以下代码是创建 k3s 集群的示例。这个命令使用了阿里云`alibaba`作为云提供商，在阿里云上创建了一个名为 “myk3s”的集群，并为该集群配置了 1 个 master 节点和 1 个 worker 节点。

```bash
autok3s -d create -p alibaba --name myk3s --master 1 --worker 1
```

<<<<<<< HEAD
### 添加 k3s 节点

以下代码是为已有的 k3s 集群添加 k3s 节点的示例。名为“myk3s”的集群是已经运行在阿里云上 的 k3s 集群。这个命令使用了阿里云`alibaba`作为云提供商，为“myk3s”集群添加了 1 个 worker 节点。

```bash
autok3s -d join --provider alibaba --name myk3s --worker 1
```

### 总结

`autok3s create` 命令的表达式可以抽象概括为：

```bash
autok3s -d create -p <云服务提供商> --name <集群名称> --master <master节点数量> --worker <worker节点数量>
```

`autok3s join`命令的表达式可以抽象概括为：

```bash
autok3s -d join -p <云服务提供商> --name <集群名称> --master <master节点数量> --worker <worker节点数量>
```

其中，`-p <云服务提供商>`和`--name <集群名称>`为必填项，用于指定云服务提供商和添加的集群名称；`--master <master节点数量>`，`--worker <worker节点数量>`为选填项，用于指定添加的节点数量，如果您只需要单独添加 master 或 worker 节点，则可以不填写另一个类型节点的参数和指定这个类型的节点数量。
=======
## 快速体验 UI

如果您不想使用命令行工具体验，可以使用`autok3s`内置 UI 来体验相关功能，请运行命令 `autok3s serve`
>>>>>>> 06d3c89... feat(autok3s): support hosted UI and API

## 演示视频

本视频展示了 autok3s 在阿里云的 ECS 实例上安装 k3s 的过程和步骤，autok3s 只需要 1 分钟左右就能将 Kubernetes (k3s)安装到阿里云的 ECS 实例上。

观看演示：

[![asciicast](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg.svg)](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg)

## 开发者指南

使用 `Makefile` 管理项目的编译、测试与打包。
项目支持使用 `dapper`，`dapper`安装步骤请参考[dapper](https://github.com/rancher/dapper)。

- 依赖：`GO111MODULE=on go mod vendor`
- 编译：`BY=dapper make autok3s`
- 测试：`BY=dapper make autok3s unit`
- 打包：`BY=dapper make autok3s package only`

## 开源协议

Copyright (c) 2020 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
