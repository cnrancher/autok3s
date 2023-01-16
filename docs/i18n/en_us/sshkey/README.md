# SSH Key Management

## Introduction

This function is to manage the SSH keys used to provision the K3s cluster.

## Base commands

The new sub-command `autok3s sshkey` is added to the command list and the SSH key management commands are under this sub-command. Here are the help docs for ssh-key management commands:

```sh
Usage:
  autok3s sshkey [command]

Available Commands:
  create      Create a new sshkey pair
  export      export the specificed ssh key pair to files
  list        List all stored ssh key pairs.
  remove      Remove a stored ssh key pair.

Flags:
  -h, --help   help for sshkey

Global Flags:
  -d, --debug   Enable log debug level

Global Environments:
  AUTOK3S_CONFIG  Path to the cfg file to use for CLI requests (default ~/.autok3s)
  AUTOK3S_RETRY   The number of retries waiting for the desired state (default 20)

Use "autok3s sshkey [command] --help" for more information about a command.
```

## Create/Generating new SSH key

If you have an existing SSH key, you can import it into autok3s by following commands:

```bash
# to import a key pair with the private key ./id_rsa and the public key ./id_rsa.pub
autok3s sshkey create import --key ./id_rsa --public-key ./id_rsa.pub

ssh key import loaded
```

If you want a new ssh key, you can generate a new one by following commands:

```bash
# create a dir to store the generated ssh key
mkdir ./certs
# generate a new ssh key
autok3s sshkey create generated -g -b 2048 -o ./certs

generating RSA ssh key pair with 2048 bit size...
ssh key generated generated
ssh key generated is written to directory ./certs
# see the generated ssh key files
ls -al ./certs

total 16
drwxr-xr-x   4 root  root   128  1 16 14:31 .
drwxr-xr-x  28 root  root   896  1 16 14:15 ..
-rw-------   1 root  root  1679  1 16 14:31 id_rsa
-rw-------   1 root  root   381  1 16 14:31 id_rsa.pub
```

And list the stored ssh keys:

```bash
autok3s sshkey list

    NAME     ENCRYPTED
  generated  false
  import     false
```

## Using stored SSH key to provision K3s cluster

You can use following commands to provision a new K3s cluster with the stored `import` SSH key via setting the parameter `--ssh-key-name`:

```bash
autok3s create -p native -n  demo --k3s-version v1.24.9+k3s1 --ssh-key-name import --master-ips 192.168.31.145
```
