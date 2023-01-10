# Airgap Support in autok3s

## Introduction

This function is to manage the airgap package for k3s airgap installation.

## Base commands

The new sub-command `autok3s airgap` is added to the command list and the package management commands are under this sub-command. Here are the help docs for airgap commands:

```sh
Usage:
  autok3s airgap [command]

Available Commands:
  create                Create a new airgap package and will download related resources from internet.
  export                export package to a tar.gz file, path can be a specific filename or a directory.
  import                Import an existing tar.gz file of airgap package. Please refer to export command
  ls                    List all stored airgap packages.
  remove                Remove a stored airgap package.
  update                Update a stored package with new selected archs.
  update-install-script Will update the embed k3s install.sh script.

Flags:
  -h, --help   help for airgap

Global Flags:
  -d, --debug                          Enable log debug level
      --log-flush-frequency duration   Maximum number of seconds between log flushes (default 5s)

Global Environments:
  AUTOK3S_CONFIG  Path to the cfg file to use for CLI requests (default ~/.autok3s)
  AUTOK3S_RETRY   The number of retries waiting for the desired state (default 20)

Use "autok3s airgap [command] --help" for more information about a command.
```

## Creating a new package

With the command `autok3s airgap create <name>`, the user can create a new package. Flags `arch` and `k3s-version` are required. Or you can input those flags with interact mode when not specified in create command.  
You can easily find the k3s version in the Github release. One or more arches can be selected from the following options: `amd64`, `arm64`, `arm`, and `s390x`. The `s390x` arch will only be supported in the recent k3s version and autok3s will throw an error if the arch is not provided in the selected k3s version.

After the package is created and validated, autok3s will start the download process from the configured site. For now, the default download site is `github` which is from the Github release page. You can change to `aliyunoss` which is from the Aliyun OSS k3s mirror via setting `/v1/settings/package-download-source` API with `autok3s serve` command. The config modification from CLI will be supported in the feature version.

The downloaded resource will be stored in `<config-path>/pakcage/<name>` and the file struct will be following:

```sh
.
├── .done
├── amd64
│   ├── .done
│   ├── k3s
│   ├── k3s-airgap-images.tar.gz
│   ├── k3s-images.txt
│   └── sha256sum.txt
├── arm64
│   ├── .done
│   ├── k3s
│   ├── k3s-airgap-images.tar.gz
│   ├── k3s-images.txt
│   └── sha256sum.txt
└── version.json
```

Each arch for your package will have its directory and will be scp to the target node when installation.

After the resources are downloaded, the package state will be `Active` and you can check it via `autok3s airgap ls` command.

```sh
    NAME     K3SVERSION      arches     STATE
  test      v1.23.9+k3s1  amd64,arm64  Active
  testtest  v1.23.9+k3s1  amd64,arm64  Active
```

## Updating a Package

The airgap package can be updated via `autok3s airgap update <name> [flags]` command. Like package create, flags `arch` and `k3s-version` are also supported.  
When you are changing the current package's k3s version or deleting a selected arch, it will require confirmation or you can use `-f, --force` to force update.

The downloaded and validated arch won't be changed after the package update and only the removed arch resources(or remove all arches when changing k3s version) will be removed.

## Import & Export

The airgap package can be exported via `autok3s airgap export <name> <path>` command. The name and path parameters are required and the path can be a specific filename with `tar.gz` suffix or can be a directory. The exported filename will be `<name>.tar.gz` if the path is a directory.  
The exported package can be imported via `autok3s airgap import <path> [name]` command or it can be used to create a k3s cluster offline with the cluster create command.

## About K3s install script

Refer to the [k3s docs](https://docs.k3s.io/installation/airgap#prerequisites), the install.sh needs to be downloaded and run in k3s node when install with airgap mode.  
In autok3s, we will have an initial install script downloaded from `https://get.k3s.io` in the CICD process and the script is stored in settings `install-script`. The command `autok3s airgap update-install-script` can be used to update the stored install script. It will download from the configured source(settings `install-script-source-repo`).

## How to create/upgrade cluster with airgap package

As you can see from the help message `./autok3s create -p native --help`, the airgap package options are added to all providers(except for `k3d`).

```sh
...
      --package-name string              The airgap package name from managed package list
      --package-path string              The airgap package path. The "package-name" flag will be ignored if this flag is also provided
...
```

You can either use `--package-name` flag with the name of the stored package, or use `--package-path` with the exported package filename.

If one of the above flags is specified, autok3s will install k3s cluster with airgap mode. The `--k3s-channel`, `--k3s-install-mirror`, `--k3s-install-script` and `--k3s-version` won't take affect. And the k3s version will be set to the package's k3s version.

The airgap installation process is like this:

- create nodes if necessary
- ssh to the node and find out the node's arch via `uname -a`
- to check node's arch is in the package included arch list or not
- scp `install.sh`, `k3s` and `k3s-image-list.tar` to the target node.
- use the airgap install command instead of the online one.

> For now, the airgap installation doesn't support docker runtime and it will be supported in the feature version.
