# autok3s
[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnrancher/autok3s)](https://goreportcard.com/report/github.com/cnrancher/autok3s) 
![GitHub release](https://img.shields.io/github/v/release/cnrancher/autok3s.svg?color=default)
[![License: apache-2.0](https://img.shields.io/badge/License-apache2-default.svg)](https://opensource.org/licenses/Apache-2.0)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://github.com/cnrancher/autok3s/pulls)

AutoK3s is a lightweight tool for quickly creating and managing K3s clusters on multiple cloud providers.
It can help users quickly complete the personalized configuration of the K3s cluster, while providing convenient kubectl access capabilities.

Other languages:
- [English](README.md)
- [Chinese Simplified (中文简体)](README_zhCN.md)

## Design Ideas
This tool uses the cloud provider's SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host.
You can also use it to join hosts as masters/agents to the K3s cluster. It will automatically merge and store the `kubeconfig` in `$HOME/.autok3s/.kube/config` which necessary for user to access the cluster.
Then user can use `autok3s kubectl` command quickly access the cluster.

Use [viper](https://github.com/spf13/viper) to bind flags and configuration file. `autok3s` will generate a configuration file to store cloud-providers' access information at the specified location(`$HOME/.autok3s/config.yaml`) to reduce the number of flags to be passed for multiple runs.

It's also generated a state file `$HOME/.autok3s/.state` to record the clusters' information created on this host.

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
