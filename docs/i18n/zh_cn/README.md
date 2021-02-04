# autok3s

[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnrancher/autok3s)](https://goreportcard.com/report/github.com/cnrancher/autok3s)
![GitHub release](https://img.shields.io/github/v/release/cnrancher/autok3s.svg?color=blue)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?color=blue)](http://github.com/cnrancher/autok3s/pulls)

简体中文 / [English](../../../README.md)

K3s是经过完全认证的Kubernetes产品，在某些情况下可以替代完整的K8s。

AutoK3s是用于简化K3s集群管理的轻量级工具，您可以使用Autok3s在任何地方运行K3s，Run K3s Everywhere。

<!-- toc -->

- [关键特性](#关键特性)
- [支持的云提供商](#支持的云提供商)
- [快速体验](#快速体验)
- [演示视频](#演示视频)
- [开发者指南](#开发者指南)

<!-- /toc -->

## 关键特性

- 通过API，CLI和UI等方式快速创建K3s
- 云提供商集成（简化每个云的[CCM](https://kubernetes.io/docs/concepts/architecture/cloud-controller)设置）
- 灵活的安装选项，例如K3s群集HA和数据存储（内置etcd，RDS，SQLite等）
- 低成本（尝试每个云中的竞价实例）
- 通过UI简化操作
- 多云之间弹性迁移，借助诸如[backup-restore-operator](https://github.com/rancher/backup-restore-operator)这样的工具

## 支持的云提供商

Autok3s可以支持以下云厂商，我们会根据社区反馈添加更多支持：

- [alibaba](alibaba/README.md) - 在阿里云的 ECS 中初始化 k3s 集群
- [tencent](tencent/README.md) - 在腾讯云 CVM 中初始化 K3s 集群
- [native](native/README.md) - 在任意类型 VM 实例中初始化 K3s 集群
- [aws](aws/README.md) - 在亚马逊 EC2 中初始化 K3s 集群

## 快速体验

Autok3s可以两种不同的运行模式：Local Mode 和 Rancher Mode。

### Local Mode

在此模式下，可以使用CLI或本地UI。

以下命令使用Alibaba ECS Provider，请在运行之前检查[前提条件](alibaba/README.md)：

```bash
export ECS_ACCESS_KEY_ID='<Your access key ID>'
export ECS_ACCESS_KEY_SECRET='<Your secret access key>'

autok3s -d create -p alibaba --name myk3s --master 1 --worker 1
```

如果要启用本地UI，请运行 `autok3s serve` 。

![autok3s-local-ui](../../assets/autok3s-local-ui.png)

### Rancher Mode

在这种模式下，您可以将Autok3s放入[Rancher](https://github.com/rancher/rancher)。
它将作为Rancher的扩展，使您可以构建一套托管K3s服务。

Autok3s创建的K3s集群可以自动导入Rancher，并充分利用Rancher的Kubernetes管理功能。

此模式正在开发中。

## 演示视频

在以下演示中，我们将在1分钟左右的时间内把K3s安装到 Alibaba ECS 云主机上。

观看演示：

[![asciicast](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg.svg)](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg)

## 开发者指南

使用 `Makefile` 管理项目的编译、测试与打包。
项目支持使用 `dapper`，`dapper`安装步骤请参考[dapper](https://github.com/rancher/dapper)。

- 依赖： `GO111MODULE=on go mod vendor`
- 编译： `BY=dapper make autok3s`
- 测试： `BY=dapper make autok3s unit`
- 打包： `BY=dapper make autok3s package only`

