# autok3s
[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)
[![License: apache-2.0](https://img.shields.io/badge/License-apache2-default.svg)](https://opensource.org/licenses/Apache-2.0)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](http://github.com/cnrancher/autok3s/pulls)

AutoK3s is a lightweight tool for quickly creating and managing K3s clusters on multiple cloud providers.
It can help users quickly complete the personalized configuration of the K3s cluster, while providing convenient kubectl access capabilities.

## Design Ideas
This tool uses the cloud provider's SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host.
You can also use it to join hosts as agents to the K3s cluster. It will automatically merge and store the `kubeconfig` in `/var/lib/rancher/autok3s/.kube/config` which necessary for user to access the cluster.
Then user can use `autok3s kubectl` command quickly access the cluster.

Use [viper](https://github.com/spf13/viper) to bind flags and configuration file. `autok3s` will generate a configuration file to store cloud-providers' access information at the specified location(`/var/lib/rancher/autok3s/config.yaml`) to reduce the number of flags to be passed for multiple runs.

It's also generated a state file `/var/lib/rancher/autok3s/.state` to record the clusters' information created on this host.

## Providers
- alibaba

## Usage
The user can get the commands available for different providers according to the `--help` flag.

### Setup K3s Cluster
If already have access information in `/var/lib/rancher/autok3s/config.yaml` you can use the simplified command.
```bash
sudo autok3s create \
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
sudo autok3s create \
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

HA(embedded etcd) mode need `--master` at least 3 nodes, e.g.
```bash
--master 2"
```

HA(external database) mode need `--master` greater than 1 node, also need to specify `--datastore` and `--k3s-version`, e.g.
```bash
--master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>" --k3s-version="v1.18.9-k3s1"
```

### Join K3s Nodes
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```bash
sudo autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --ssh-key-path <ssh-key-path> \
    --worker 1
```

Generic commands can be used anywhere.
```bash
sudo autok3s join \
    --provider alibaba \
    --region <region> \
    --name <cluster name> \
    --key-pair <key-pair id> \
    --v-switch <v-switch id> \
    --security-group <security-group id> \
    --token <k3s token> \
    --ip <k3s master/lb ip> \
    --worker 1
```

Join master nodes to (embedded etcd) HA cluster e.g.
```bash
--master 2
```

Join master nodes to (external database) HA cluster, also need to specify `--datastore` and `--k3s-version`, e.g.
```bash
--master 2 --datastore "mysql://<user>:<password>@tcp(<ip>:<port>)/<db>" --k3s-version="v1.18.9-k3s1"
```

### Delete K3s Cluster
```bash
sudo autok3s delete \
    --provider alibaba \
    --region <region> \
    --name <cluster name>
```

### List K3s Clusters
This command will list the clusters that you have created on this machine.
```bash
sudo autok3s list
```

### Access K3s Cluster
After cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.
```bash
sudo autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.
```bash
sudo autok3s kubectl config get-contexts
sudo autok3s kubectl config use-context <context>
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
