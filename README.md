# autok3s
[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnrancher/autok3s)](https://goreportcard.com/report/github.com/cnrancher/autok3s) 
![GitHub release](https://img.shields.io/github/v/release/cnrancher/autok3s.svg)
[![License: apache-2.0](https://img.shields.io/badge/License-apache2-default.svg?color=blue)](https://opensource.org/licenses/Apache-2.0)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?color=blue)](http://github.com/cnrancher/autok3s/pulls)

English / [简体中文](README_zhCN.md)

AutoK3s is a lightweight tool for quickly creating and managing K3s clusters on multiple cloud providers.
It can help users quickly complete the personalized configuration of the K3s cluster, while providing convenient kubectl access capabilities.

## Key Features
- A simple command can quickly generate and manage a custom k3s cluster.
- Speed up the process of creating k3s instances on the public cloud platform.
- Support for all the functions of Kubectl.
- Support for enabling Kubernetes Cloud-Controller-Manager.
- Support for enabling Kubernetes Dashboard UI.
- Support for enabling additional public cloud platform's CNI plugin(e.g. Terway).

## Providers
See the providers' links below for more usage details:

- [alibaba](docs/alibaba/README.md) - Uses Alibaba Cloud SDK manage hosts, then uses SSH to install or join K3s cluster and hosts.
- [tencent](docs/tencent/README.md) - Uses Tencent Cloud SDK manage hosts, then uses SSH to install or join K3s cluster and hosts.
- [native](docs/native/README.md) - Does not integrate the Cloud SDK, but only uses SSH to install or join K3s cluster and hosts.

## Demo Video
Using `autok3s -d create --provider alibaba` command create K3s cluster.

[![asciicast](https://asciinema.org/a/whwyjSfGv7lZdjaenTDCRejDW.svg)](https://asciinema.org/a/whwyjSfGv7lZdjaenTDCRejDW)

## Developers' Guide
Use `Makefile` to manage project compilation, testing and packaging.
Of course, you can also choose to compile using `dapper`.
Install `dapper` please follow the [dapper](https://github.com/rancher/dapper) project.

- vendor: `GO111MODULE=on go mod vendor`
- compilation: `BY=dapper make autok3s`
- testing: `BY=dapper make autok3s unit`
- packing: `BY=dapper make autok3s package only`

# License

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
