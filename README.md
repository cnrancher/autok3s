# autok3s
[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![License: apache-2.0](https://img.shields.io/badge/License-apache2-default.svg)](https://opensource.org/licenses/Apache-2.0)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://github.com/cnrancher/autok3s/pulls)

AutoK3s is a lightweight tool for quickly creating and managing K3s clusters on multiple cloud providers.
It can help users quickly complete the personalized configuration of the K3s cluster, while providing convenient kubectl access capabilities.

## Design Ideas
This tool uses the cloud provider's SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host.
You can also use it to join hosts as masters/agents to the K3s cluster. It will automatically merge and store the `kubeconfig` in `$HOME/.autok3s/.kube/config` which necessary for user to access the cluster.
Then user can use `autok3s kubectl` command quickly access the cluster.

Use [viper](https://github.com/spf13/viper) to bind flags and configuration file. `autok3s` will generate a configuration file to store cloud-providers' access information at the specified location(`$HOME/.autok3s/config.yaml`) to reduce the number of flags to be passed for multiple runs.

It's also generated a state file `$HOME/.autok3s/.state` to record the clusters' information created on this host.

## Providers
- alibaba
- [native](docs/native/README.md)

## Pre-Requests
The following demo uses the Alibaba Provider, so you need to set the following [RAMs](docs/alibaba/ram.md).

## Usage
User can get the commands available for different providers according to the `--provider <provider> --help`.

### Setup K3s Cluster
If already have access information in `$HOME/.autok3s/config.yaml` you can use the simplified command.
```bash
autok3s create \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --ssh-key-path <ssh-key-path> \
    --master 1
```

Generic commands can be used anywhere.
```bash
autok3s create \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --ssh-key-path <ssh-key-path> \
    --access-key <access-key> \
    --access-secret <access-secret> \
    --master 1
```

SSH connection also supports the following options.

| Option Name | Description |
| --- | --- |
| --ssh-user | SSH user for host |
| --ssh-port | SSH port for host |
| --ssh-key-path | SSH private key path |
| --ssh-key-pass | SSH passphrase of private key |
| --ssh-key-cert-path | SSH private key certificate path |
| --ssh-password | SSH login password |

HA(embedded etcd: >= 1.19.1-k3s1) mode need `--master` at least 3 master nodes, e.g.
```bash
autok3s ... \
    --master 3
```

HA(external database) mode need `--master` greater than 1 node, also need to specify `--datastore`, e.g.
```bash
autok3s ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Join K3s Nodes
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --worker 1
```

Generic commands can be used anywhere.
```bash
autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --token <k3s token> \
    --ip <k3s master/lb ip> \
    --access-key <access-key> \
    --access-secret <access-secret> \
    --worker 1
```

Join master nodes to (embedded etcd: >= 1.19.1-k3s1) HA cluster e.g.
```bash
autok3s ... \
    --master 2
```

Join master nodes to (external database) HA cluster, also need to specify `--datastore`, e.g.
```bash
autok3s ... \
    --master 2 \
    --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>"
```

### Start K3s Cluster
> Provider `native` **DO NOT** support.

If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s start \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s start \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
```

### Stop K3s Cluster
> Provider `native` **DO NOT** support.

If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s stop \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
```

### Delete K3s Cluster
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --access-key <access-key> \
    --access-secret <access-secret>
```

### List K3s Clusters
This command will list the clusters that you have created on this machine.
```bash
autok3s list
```

### Access K3s Cluster
After the cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.
```bash
autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.
```bash
autok3s kubectl config get-contexts
autok3s kubectl config use-context <context>
```

### SSH K3s Cluster's Node
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
autok3s ssh \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

Generic commands can be used anywhere.
```bash
autok3s ssh \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh private key path> \
    --ssh-user root \
    --ssh-port 22 \
    --access-key <access-key> \
    --access-secret <access-secret>
```

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
