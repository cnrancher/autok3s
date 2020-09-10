# autok3s
[![Build Status](http://drone-pandaria.cnrancher.com/api/badges/cnrancher/autok3s/status.svg)](http://drone-pandaria.cnrancher.com/cnrancher/autok3s)

AutoK3s is a lightweight tool for quickly creating and managing K3s clusters on multiple cloud providers.
It can help users quickly complete the personalized configuration of the K3s cluster, while providing convenient kubectl access capabilities.

## Design Ideas
This tool uses the cloud provider's SDK to create and manage hosts, and then uses SSH to install the K3s cluster to the remote host.
You can also use it to join hosts as agents to the K3s cluster. It will automatically merge and store the `kubeconfig` in `/var/lib/rancher/autok3s/.kube/config` which necessary for user to access the cluster.
Then user can use `autok3s kubectl` command quickly access the cluster.

Use [viper](https://github.com/spf13/viper) to bind flags and configuration file. `autok3s` will generate a configuration file to store cloud-providers' access information at the specified location(`/var/lib/rancher/autok3s/config.yaml`) to reduce the number of flags to be passed for multiple runs.
```
autok3s:
  providers:
    alibaba:
      access-key: <access-key>
      access-secret: <access-secret>
```

It's also generated a state file `/var/lib/rancher/autok3s/.state` to record the clusters' information created on this host.

## Providers
- alibaba - currently supported
  - accelerate the image pull speed
  - support enable terway eni mode
  - support enable alibaba cloud-controller-manager
  - support enable ui components(octopus/kubernetes dashboard)

## Usage
The user can get the commands available for different providers according to the `--help` flag.

### Setup K3s Cluster
If already have access information in `/var/lib/rancher/autok3s/config.yaml` you can use the simplified command.
```
autok3s create --provider alibaba --region cn-hangzhou --name <cluster name> --key-pair <key-pair id> --v-switch <v-switch id> --security-group <security-group id> --ssh-key-path <ssh-key-path>
```

Generic commands can be used anywhere.
```
autok3s create --provider alibaba --region cn-hangzhou --name <cluster name> --key-pair <key-pair id> --v-switch <v-switch id> --security-group <security-group id> --ssh-key-path <ssh-key-path> --access-key <access-key> --access-secret <access-secret>
```

By default, one master and one worker will be created, and the number can be changed by `--worker`.

### Join K3s Nodes
If you have ever created a cluster using `autok3s` on your current machine, you can use the simplified command.
```
autok3s join --provider alibaba --name <cluster name>
```

Generic commands can be used anywhere.
```
autok3s join --provider alibaba --region cn-hangzhou --name <cluster name> --key-pair <key-pair id> --v-switch <v-switch id> --security-group <security-group id> --token <k3s token> --url <k3s master ip>
```

By default, one worker will be created, and the number can be changed by `--worker`.

### List K3s Clusters
This command will list the clusters that you have created on this machine.
```
autok3s get cluster
```

### Access K3s Cluster
After cluster created, `autok3s` will automatically merge the `kubeconfig` which necessary for us to access the cluster.
```
autok3s kubectl <sub-commands> <flags>
```

In the scenario of multiple clusters, the access to different clusters can be completed by switching context.
```
autok3s kubectl config context get-contexts
autok3s kubectl config context use-context <context>
```

## Developers' Guide
Use `Makefile` to manage project compilation, testing and packaging.
Of course, you can also choose to compile using `dapper`.
Install `dapper` please follow the [dapper](https://github.com/rancher/dapper) project. PRs are welcome.

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
