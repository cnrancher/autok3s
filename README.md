# autok3s
[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnrancher/autok3s)](https://goreportcard.com/report/github.com/cnrancher/autok3s) 
![GitHub release](https://img.shields.io/github/v/release/cnrancher/autok3s.svg)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?color=blue)](http://github.com/cnrancher/autok3s/pulls)

English / [简体中文](docs/i18n/zh_cn/README.md)

AutoK3s is a lightweight tool for quickly creating and managing k3s clusters on multiple cloud providers.
It can help users quickly complete the personalized configuration of the k3s cluster, while providing convenient kubectl access capabilities.

## Key Features
- Bootstrap Kubernetes with k3s onto multiple cloud providers with `autok3s create`
- Join nodes into an existing k3s cluster with `autok3s join`
- Automatically generate `kubeconfig` file for the cluster which you created
- Integrate `kubectl` to provide access to the cluster
- Bootstrap a HA Kubernetes with k3s cluster
- Provide additional option to enable Kubernetes Cloud-Controller-Manager with `--cloud-controller-manager`
- Provide additional option to enable Kubernetes Dashboard UI with `--ui`
- Provide additional option to enable cloud platform's CNI plugin, e.g `--terway 'eni'`

## Providers
See the providers' links below for more usage details:

- [alibaba](docs/i18n/en_us/alibaba/README.md) - Bootstrap Kubernetes with k3s onto Alibaba ECS
- [tencent](docs/i18n/en_us/tencent/README.md) - Bootstrap Kubernetes with k3s onto Tencent CVM
- [native](docs/i18n/en_us/native/README.md) - Bootstrap Kubernetes with k3s onto any VM

## Quick Start
The following command use the `alibaba` provider, with prerequisites that refer to the [alibaba](docs/i18n/en_us/alibaba/README.md) document.

```bash
export ECS_ACCESS_KEY_ID='<Your access key ID>'
export ECS_ACCESS_KEY_SECRET='<Your secret access key>'

autok3s -d create -p alibaba --name myk3s --master 1 --worker 1
```

## Demo Video
The demo install Kubernetes (k3s) onto Alibaba ECS machines in around 1 minutes.

Watch the demo:

[![asciicast](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg.svg)](https://asciinema.org/a/EL5P2ILES8GAvdlhaxLMnY8Pg)

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
