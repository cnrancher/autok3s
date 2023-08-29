# Add-on Support in AutoK3s

## Introduction

This section introduces the management of AutoK3s Add-ons and demonstrates how to utilize add-ons to deploy applications after creating a cluster.

## Base Command

A new sub-command, autok3s add-ons, has been added to the command list. This command is used to manage additional plugins. Here are the help docs for the add-ons commands:
```sh
Usage:
  autok3s add-ons [command]

Available Commands:
  create      Create a new add-on
  get         Get an add-on information.
  list        List all add-on list.
  rm          Remove an add-on.
  update      Update manifest for an add-on

Flags:
  -h, --help   help for add-ons

Global Flags:
  -d, --debug   Enable log debug level

Global Environments:
  AUTOK3S_CONFIG  Path to the cfg file to use for CLI requests (default ~/.autok3s)
  AUTOK3S_RETRY   The number of retries waiting for the desired state (default 20)

Use "autok3s add-ons [command] --help" for more information about a command.
```

### Creating a new Add-on

You can create a new add-on using the `autok3s add-ons create <name>` command. The `--from` or `-f` parameter is mandatory to specify the path of the Manifest YAML file for the add-on. 
The Manifest YAML file content can either be an existing [Helm Chart](https://docs.k3s.io/zh/helm) or YAML of K8s resources, such as the built-in [local-storage add-on](https://github.com/k3s-io/k3s/blob/master/manifests/local-storage.yaml) in K3s.

```sh
autok3s add-ons create my-ns -f ~/myapp.yaml --description "my namespace" --set name=test01 --set creator=jacie
```

You can use the `--set` parameter similarly to Helm, to replace variables defined in the YAML.

AutoK3s natively supports the Rancher Manager add-on. Users can directly deploy Rancher Manager with a local K3s cluster.

### Updating an Add-on

To update an add-on, use the `autok3s add-ons update <name>` command. Similar to the create command, you can use `--from` or `-f` to replace the content of the manifest file. You can also use `--unset` to remove specific values.

```sh
autok3s add-ons update rancher --unset Version
```

### Listing Add-ons

Use the `autok3s add-ons list` command to list all available add-ons.

```
   NAME             DESCRIPTION            VALUES
  rancher  Default Rancher Manager add-on  0
  my-ns    my namespace                    2
```

### Describing an Add-on

Retrieve detailed information about an add-on using the `autok3s add-ons get <name>` command.

```sh
Description: my namespace
Manifest: |
  apiVersion: v1
  kind: Namespace
  metadata:
    name: {{ .name | default "myns"}}
    label:
      owner: {{ .creator | default "" }}
Name: my-ns
Values:
  creator: jacie
  name: test01
```

### Deleting an add-on

Use the `autok3s add-ons rm <name>` command to delete an add-on.

### Deploying a Cluster with Add-ons

When creating a cluster using AutoK3s, you can enable add-ons on K3s by specifying `--enable`. This action will automatically deploy the specified applications after the K3s cluster starts.

```sh
autok3s create -p aws -n myk3s \
    ... \
    --enable rancher \
    --enable my-ns \
    --set rancher.Version=v2.7.2 \
    --set my-ns.name=test
```

You can use the `--enable` flag to specify multiple add-ons. The add-on names must match those in the add-on management. 

The `--set` parameter should specify the prefix of the add-on name to differentiate between different add-on parameter values. If values are already set for the add-on, they can be omitted during cluster creation. If values are specified, they will be used as the final replacement content.
