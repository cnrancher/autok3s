# autok3s
[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnrancher/autok3s)](https://goreportcard.com/report/github.com/cnrancher/autok3s) 
![GitHub release](https://img.shields.io/github/v/release/cnrancher/autok3s.svg?color=blue)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?color=blue)](http://github.com/cnrancher/autok3s/pulls)

简体中文 / [English](../../../README.md)

快速创建并启动k3s集群，同时可以使用它来为k3s集群添加节点，提升公有云体验的同时，继承kubectl从而提供便捷的集群能力。

<!-- toc -->

- [关键特性](#关键特性)
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
- 集成扩展参数 `例如 --terway 'eni'` 以开启公有云 CNI 网络插件

## 支持的云提供商
有关更多用法的详细信息，请参见下面的链接：

- [alibaba](alibaba/README.md) - 在阿里云的 ECS 中初始化 Kubernetes (k3s) 集群
- [tencent](tencent/README.md) - 在腾讯云 CVM 中初始化 Kubernetes (k3s) 集群
- [native](native/README.md) - 在任意类型 VM 实例中初始化 Kubernetes (k3s) 集群
- [aws](aws/README.md) - 在亚马逊 EC2 中初始化 Kubernetes (k3s) 集群

## 快速体验
以下命令使用`alibaba`作为云提供商，相关的前置条件请参考[alibaba](alibaba/README.md)云提供商文档。

```bash
export ECS_ACCESS_KEY_ID='<Your access key ID>'
export ECS_ACCESS_KEY_SECRET='<Your secret access key>'

autok3s -d create -p alibaba --name myk3s --master 1 --worker 1
```

## 快速体验 UI

如果您不想使用命令行工具体验，可以使用`autok3s`内置 UI 来体验相关功能，请运行命令 `autok3s serve`

## 演示视频
示程序在1分钟左右就能将Kubernetes (k3s)安装到阿里云的ECS实例上。

观看演示:

[![asciicast](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg.svg)](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg)

## 开发者指南
使用 `Makefile` 管理项目的编译、测试与打包。
项目支持使用 `dapper`，`dapper`安装步骤请参考[dapper](https://github.com/rancher/dapper)。

- 依赖: `GO111MODULE=on go mod vendor`
- 编译: `BY=dapper make autok3s`
- 测试: `BY=dapper make autok3s unit`
- 打包: `BY=dapper make autok3s package only`

# 开源协议

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
