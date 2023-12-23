[![Build Status](https://github.com/Fred78290/kubernetes-aws-autoscaler/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/Fred78290/Fred78290_kubernetes-aws-autoscaler/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-aws-autoscaler&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-aws-autoscaler)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-aws-autoscaler/blob/master/LICENSE)

# kubernetes-aws-autoscaler

Kubernetes autoscaler for aws

## Supported releases

* 1.26.11
  * This version is supported kubernetes v1.26 and support k3s, rke2, external kubernetes distribution
* 1.27.9
  * This version is supported kubernetes v1.27 and support k3s, rke2, external kubernetes distribution
* 1.28.4
  * This version is supported kubernetes v1.28 and support k3s, rke2, external kubernetes distribution
* 1.29.0
  * This version is supported kubernetes v1.29 and support k3s, rke2, external kubernetes distribution

## How it works

This tool will drive AWS to deploy EC2 instance at the demand. The cluster autoscaler deployment use an enhanced version of cluster-autoscaler. <https://github.com/Fred78290/autoscaler>.

This version use grpc to communicate with the cloud provider hosted outside the pod. A docker image is available here <https://hub.docker.com//fred78290/cluster-autoscaler>

You can also use the vanilla autoscaler with the [externalgrpc cloud provider](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler/cloudprovider/externalgrpc)

A sample of the cluster-autoscaler deployment is available at [examples/cluster-autoscaler.yaml](./examples/kubeadm/cluster-autoscaler.yaml). You must fill value between <>

### Before you must create a kubernetes cluster on AWS

You can do it from scrash or you can use script from projetct [autoscaled-masterkube-aws](https://github.com/Fred78290/autoscaled-masterkube-aws)  to create a kubernetes cluster in single control plane or in HA mode with 3 control planes.

## Commandline arguments

| Parameter | Description |
| --- | --- |
| `version` | Print the version and exit  |
| `save`  | Tell the tool to save state in this file  |
| `config`  |The the tool to use config file |

## Build

The build process use make file. The simplest way to build is `make container`

# New features

## Use k3s, rke2 or external as kubernetes distribution method

Instead using **kubeadm** as kubernetes distribution method, it is possible to use **k3s**, **rke2** or **external**

**external** allow to use custom shell script to join cluster

Samples provided here

* [kubeadm](./examples/kubeadm/cluster-autoscaler.yaml)
* [rke2](./examples/rke2/cluster-autoscaler.yaml)
* [k3s](./examples/k3s/cluster-autoscaler.yaml)
* [external](./examples/external/cluster-autoscaler.yaml)

## Use the vanilla autoscaler with extern gRPC cloud provider

You can also use the vanilla autoscaler with the [externalgrpc cloud provider](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler/cloudprovider/externalgrpc)

Samples of the cluster-autoscaler deployment with vanilla autoscaler. You must fill value between <>

* [kubeadm](./examples/kubeadm/cluster-autoscaler-vanilla.yaml)
* [rke2](./examples/rke2/cluster-autoscaler-vanilla.yaml)
* [k3s](./examples/rke2/cluster-autoscaler-vanilla.yaml)
* [external](./examples/external/cluster-autoscaler-vanilla.yaml)

## Use external kubernetes distribution

When you use a custom method to create your cluster, you must provide a shell script to vmware-autoscaler to join the cluster. The script use a yaml config created by vmware-autscaler at the given path.

config: /etc/default/vmware-autoscaler-config.yaml

```yaml
provider-id: vsphere://42373f8d-b72d-21c0-4299-a667a18c9fce
max-pods: 110
node-name: vmware-dev-rke2-woker-01
server: 192.168.1.120:9345
token: K1060b887525bbfa7472036caa8a3c36b550fbf05e6f8e3dbdd970739cbd7373537
disable-cloud-controller: false
````

If you declare to use an external etcd service

```yaml
datastore-endpoint: https://1.2.3.4:2379
datastore-cafile: /etc/ssl/etcd/ca.pem
datastore-certfile: /etc/ssl/etcd/etcd.pem
datastore-keyfile: /etc/ssl/etcd/etcd-key.pem
```

You can also provide extras config onto this file

```json
{
  "external": {
    "join-command": "/usr/local/bin/join-cluster.sh"
    "config-path": "/etc/default/vmware-autoscaler-config.yaml"
    "extra-config": {
        "mydata": {
          "extra": "ball"
        },
        "...": "..."
    }
  }
}
```

Your script is responsible to set the correct kubelet flags such as max-pods=110, provider-id=aws://<zone-id>/<instance-id>, cloud-provider=external, ...

## Annotations requirements

If you expected to use vmware-autoscaler on already deployed kubernetes cluster, you must add some node annotations to existing node

Also don't forget to create an image usable by vmware-autoscaler to scale up the cluster [create-image.sh](https://raw.githubusercontent.com/Fred78290/autoscaled-masterkube-vmware/master/bin/create-image.sh)

| Annotation | Description | Value |
| --- | --- | --- |
| `cluster-autoscaler.kubernetes.io/scale-down-disabled` | Avoid scale down for this node |true|
| `cluster.autoscaler.nodegroup/name` | Node group name |aws-dev-rke2|
| `cluster.autoscaler.nodegroup/autoprovision` | Tell if the node is provisionned by vmware-autoscaler |false|
| `cluster.autoscaler.nodegroup/instance-id` | The vm UUID |42373f8d-b72d-21c0-4299-a667a18c9fce|
| `cluster.autoscaler.nodegroup/instance-name` | The instance name |aws-dev-rke2-master-01|
| `cluster.autoscaler.nodegroup/managed` | Tell if the node is managed by vmware-autoscaler not autoscaled |false|
| `cluster.autoscaler.nodegroup/node-index` | The node index, will be set if missing |0|

Sample master node

```text
    cluster-autoscaler.kubernetes.io/scale-down-disabled: "true"
    cluster.autoscaler.nodegroup/autoprovision: "false"
    cluster.autoscaler.nodegroup/instance-id: 42373f8d-b72d-21c0-4299-a667a18c9fce
    cluster.autoscaler.nodegroup/instance-name: aws-dev-rke2-master-01
    cluster.autoscaler.nodegroup/managed: "false" 
    cluster.autoscaler.nodegroup/name: aws-dev-rke2
    cluster.autoscaler.nodegroup/node-index: "0"
```

Sample first worker node

```text
    cluster-autoscaler.kubernetes.io/scale-down-disabled: "true"
    cluster.autoscaler.nodegroup/autoprovision: "false"
    cluster.autoscaler.nodegroup/instance-id: 42370879-d4f7-eab0-a1c2-918a97ac6856
    cluster.autoscaler.nodegroup/managed: "false"
    cluster.autoscaler.nodegroup/name: vmware-dev-rke2
    cluster.autoscaler.nodegroup/node-index: "1"
```

Sample autoscaled worker node

```text
    cluster-autoscaler.kubernetes.io/scale-down-disabled: "false"
    cluster.autoscaler.nodegroup/autoprovision: "true"
    cluster.autoscaler.nodegroup/instance-id: 3d25c629-3f1d-46b3-be9f-b95db2a64859
    cluster.autoscaler.nodegroup/managed: "false"
    cluster.autoscaler.nodegroup/name: vmware-dev-rke2
    cluster.autoscaler.nodegroup/node-index: "2"
```

## Node labels

These labels will be added

| Label | Description | Value |
| --- | --- | --- |
|`node-role.kubernetes.io/control-plane`|Tell if the node is control-plane |true|
|`node-role.kubernetes.io/master`|Tell if the node is master |true|
|`node-role.kubernetes.io/worker`|Tell if the node is worker |true|

## Cloud provider AWS compliant

Version 1.24.6 and 1.25.2 and above are [cloud-provider-aws](https://github.com/kubernetes/cloud-provider-aws) by building provider-id conform to syntax `aws://<zone-id>/<instance-id>`

## CRD controller

This new release include a CRD controller allowing to create kubernetes node without use of aws cli or code. Just by apply a configuration file, you have the ability to create nodes on the fly.

As exemple you can take a look on [artifacts/examples/example.yaml](artifacts/examples/example.yaml) on execute the following command to create a new node

```bash
kubectl apply -f artifacts/examples/example.yaml
```

If you want delete the node just delete the CRD with the call

```bash
kubectl delete -f artifacts/examples/example.yaml
```

You have the ability also to create a control plane as instead a worker

```bash
kubectl apply -f artifacts/examples/controlplane.yaml
```

The resource is cluster scope so you don't need a namespace. The name of the resource is not the name of the managed node.

The minimal resource declaration

```yaml
apiVersion: "nodemanager.aldunelabs.com/v1alpha1"
kind: "ManagedNode"
metadata:
  name: "aws-ca-k8s-managed-01"
spec:
  nodegroup: aws-ca-k8s
  instanceType: t3a.medium
  diskSizeInGB: 10
  eni:
    subnetID: subnet-1234
    securityGroup: sg-5678
```

The full qualified resource including networks declaration to override the default controller network management and adding some node labels & annotations. If you specify the managed node as controller, you can also allows the controlplane to support deployment as a worker node

```yaml
apiVersion: "nodemanager.aldunelabs.com/v1alpha1"
kind: "ManagedNode"
metadata:
  name: "aws-ca-k8s-managed-01"
spec:
  nodegroup: aws-ca-k8s
  controlPlane: false
  allowDeployment: false
  instanceType: t3a.medium
  diskSizeInGB: 10
  labels:
  - demo-label.aldunelabs.com=demo
  - sample-label.aldunelabs.com=sample
  annotations:
  - demo-annotation.aldunelabs.com=demo
  - sample-annotation.aldunelabs.com=sample
  eni:
    subnetID: subnet-1234
    securityGroup: sg-5678
    privateAddress: 172.30.64.80
    publicIP: false
```

It's possible also to specify an existing ENI.

```yaml
apiVersion: "nodemanager.aldunelabs.com/v1alpha1"
kind: "ManagedNode"
metadata:
  name: "aws-ca-k8s-master-02"
spec:
  nodegroup: aws-ca-k8s
  controlPlane: true
  allowDeployment: false
  instanceType: t3a.medium
  diskSizeInGB: 20
  labels:
  - demo-label.aldunelabs.com=demo
  - sample-label.aldunelabs.com=sample
  annotations:
  - demo-annotation.aldunelabs.com=demo
  - sample-annotation.aldunelabs.com=sample
  eni:
    networkInterfaceID: eni-0875ac4cdac6da498
```

### Sample config

As example of use generated by autoscaled-masterkube-aws scripts [autoscaled-masterkube-aws](https://github.com/Fred78290/autoscaled-masterkube-aws)

```json
{
    "distribution": "rke2",
    "use-external-etcd": false,
    "src-etcd-ssl-dir": "/etc/kubernetes/pki/etcd",
    "dst-etcd-ssl-dir": "/etc/kubernetes/pki/etcd",
    "kubernetes-pki-srcdir": "/etc/kubernetes/pki",
    "kubernetes-pki-dstdir": "/etc/kubernetes/pki",
    "network": "unix",
    "listen": "/var/run/cluster-autoscaler/aws.sock",
    "secret": "aws",
    "minNode": 0,
    "maxNode": 9,
    "maxPods": 17,
    "node-name-prefix": "autoscaled",
    "managed-name-prefix": "managed",
    "controlplane-name-prefix": "master",
    "nodePrice": 0,
    "podPrice": 0,
    "image": "focal-k8s-rke2-aws-v1.29.0-containerd-amd64",
    "cloud-provider": "external",
    "optionals": {
        "pricing": false,
        "getAvailableMachineTypes": false,
        "newNodeGroup": false,
        "templateNodeInfo": false,
        "createNodeGroup": false,
        "deleteNodeGroup": false
    },
    "kubeadm": {
        "address": "172.30.73.121:6443",
        "token": "m1vmoc.3ox7sartsgk8f14l",
        "ca": "sha256:70ecc6f82ad1a6938d6fbd4865f4a8d0fb4fcec70cea4140dff3b657b9600c0e",
        "extras-args": [
            "--ignore-preflight-errors=All"
        ]
    },
    "k3s": {
        "address": "172.30.73.121:6443",
        "token": "m1vmoc.3ox7sartsgk8f14l",
        "datastore-endpoint": "https://1.2.3.4:2379",
        "extras-commands": []
    },
    "rke2": {
        "address": "172.30.73.121:6443",
        "token": "m1vmoc.3ox7sartsgk8f14l",
        "datastore-endpoint": "https://1.2.3.4:2379",
        "extras-commands": []
    },
    "external": {
        "address": "172.30.73.121:6443",
        "token": "m1vmoc.3ox7sartsgk8f14l",
        "datastore-endpoint": "https://1.2.3.4:2379",
        "join-command": "/usr/local/bin/join-cluster.sh",
        "config-path": "/etc/default/vmware-autoscaler-config.yaml",
        "extra-config": {
            "...": "..."
        }
    },
    "default-machine": "t3a.medium",
    "machines": {
        "t3a.nano": {
            "price": 0.0051,
            "memsize": 512,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3a.micro": {
            "price": 0.0102,
            "memsize": 1024,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3a.small": {
            "price": 0.0204,
            "memsize": 2048,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3a.medium": {
            "price": 0.0408,
            "memsize": 4096,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3a.large": {
            "price": 0.0816,
            "memsize": 8192,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3a.xlarge": {
            "price": 0.1632,
            "memsize": 16384,
            "vcpus": 4,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3a.2xlarge": {
            "price": 0.3264,
            "memsize": 32768,
            "vcpus": 8,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3.nano": {
            "price": 0.0057,
            "memsize": 512,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3.micro": {
            "price": 0.0114,
            "memsize": 1024,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3.small": {
            "price": 0.0228,
            "memsize": 2048,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3.medium": {
            "price": 0.0456,
            "memsize": 4096,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3.large": {
            "price": 0.0912,
            "memsize": 8192,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3.xlarge": {
            "price": 0.1824,
            "memsize": 16384,
            "vcpus": 4,
            "diskType": "gp2",
            "diskSize": 10
        },
        "t3.2xlarge": {
            "price": 0.3648,
            "memsize": 32768,
            "vcpus": 8,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.large": {
            "vcpus": 2,
            "memsize": 4096,
            "price": 0.101,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.xlarge": {
            "vcpus": 4,
            "memsize": 8192,
            "price": 0.202,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.2xlarge": {
            "vcpus": 8,
            "memsize": 16384,
            "price": 0.404,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.4xlarge": {
            "vcpus": 16,
            "memsize": 32768,
            "price": 0.808,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.9xlarge": {
            "vcpus": 36,
            "memsize": 73728,
            "price": 1.818,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.12xlarge": {
            "vcpus": 48,
            "memsize": 98304,
            "price": 2.424,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.18xlarge": {
            "vcpus": 72,
            "memsize": 147456,
            "price": 3.636,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5.24xlarge": {
            "vcpus": 96,
            "memsize": 196608,
            "price": 4.848,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.large": {
            "vcpus": 2,
            "memsize": 4096,
            "price": 0.091,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.xlarge": {
            "vcpus": 4,
            "memsize": 8192,
            "price": 0.182,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.2xlarge": {
            "vcpus": 8,
            "memsize": 16384,
            "price": 0.364,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.4xlarge": {
            "vcpus": 16,
            "memsize": 32768,
            "price": 0.728,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.8xlarge": {
            "vcpus": 32,
            "memsize": 65536,
            "price": 1.456,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.12xlarge": {
            "vcpus": 48,
            "memsize": 98304,
            "price": 2.184,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.16xlarge": {
            "vcpus": 64,
            "memsize": 131072,
            "price": 2.912,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c5a.24xlarge": {
            "vcpus": 96,
            "memsize": 196608,
            "price": 4.368,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5a.large": {
            "price": 0.096,
            "memsize": 8192,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5a.xlarge": {
            "price": 0.192,
            "memsize": 16384,
            "vcpus": 4,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5a.2xlarge": {
            "price": 0.384,
            "memsize": 32768,
            "vcpus": 8,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5a.4xlarge": {
            "price": 0.768,
            "memsize": 65536,
            "vcpus": 16,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5a.8xlarge": {
            "price": 1.536,
            "memsize": 131072,
            "vcpus": 32,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5a.12xlarge": {
            "price": 2.304,
            "memsize": 196608,
            "vcpus": 48,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5a.16xlarge": {
            "price": 3.072,
            "memsize": 196608,
            "vcpus": 64,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5.large": {
            "price": 0.107,
            "memsize": 8192,
            "vcpus": 2,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5.xlarge": {
            "price": 0.214,
            "memsize": 16384,
            "vcpus": 4,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5.2xlarge": {
            "price": 0.428,
            "memsize": 32768,
            "vcpus": 8,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5.4xlarge": {
            "price": 0.856,
            "memsize": 65536,
            "vcpus": 16,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5.8xlarge": {
            "price": 1.712,
            "memsize": 131072,
            "vcpus": 32,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5.12xlarge": {
            "price": 2.568,
            "memsize": 196608,
            "vcpus": 48,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m5.16xlarge": {
            "price": 3.424,
            "memsize": 196608,
            "vcpus": 64,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.large": {
            "vcpus": 2,
            "memsize": 16384,
            "price": 0.148,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.xlarge": {
            "vcpus": 4,
            "memsize": 32768,
            "price": 0.296,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.2xlarge": {
            "vcpus": 8,
            "memsize": 65536,
            "price": 0.592,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.large": {
            "vcpus": 2,
            "memsize": 16384,
            "price": 0.148,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.xlarge": {
            "vcpus": 4,
            "memsize": 32768,
            "price": 0.296,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.2xlarge": {
            "vcpus": 8,
            "memsize": 65536,
            "price": 0.592,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.4xlarge": {
            "vcpus": 16,
            "memsize": 131072,
            "price": 1.184,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.8xlarge": {
            "vcpus": 32,
            "memsize": 262144,
            "price": 2.368,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.12xlarge": {
            "vcpus": 48,
            "memsize": 393216,
            "price": 3.552,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.16xlarge": {
            "vcpus": 64,
            "memsize": 524288,
            "price": 4.736,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5.24xlarge": {
            "vcpus": 96,
            "memsize": 786432,
            "price": 7.104,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.large": {
            "vcpus": 2,
            "memsize": 16384,
            "price": 0.133,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.xlarge": {
            "vcpus": 4,
            "memsize": 32768,
            "price": 0.266,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.2xlarge": {
            "vcpus": 8,
            "memsize": 65536,
            "price": 0.532,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.4xlarge": {
            "vcpus": 16,
            "memsize": 131072,
            "price": 1.064,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.8xlarge": {
            "vcpus": 32,
            "memsize": 262144,
            "price": 2.128,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.12xlarge": {
            "vcpus": 48,
            "memsize": 393216,
            "price": 3.192,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.16xlarge": {
            "vcpus": 64,
            "memsize": 524288,
            "price": 4.256,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r5a.24xlarge": {
            "vcpus": 96,
            "memsize": 786432,
            "price": 6.384,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.4xlarge": {
            "vcpus": 16,
            "memsize": 131072,
            "price": 1.184,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.8xlarge": {
            "vcpus": 32,
            "memsize": 262144,
            "price": 2.368,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.12xlarge": {
            "vcpus": 48,
            "memsize": 393216,
            "price": 3.552,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.16xlarge": {
            "vcpus": 64,
            "memsize": 524288,
            "price": 4.736,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.24xlarge": {
            "vcpus": 96,
            "memsize": 786432,
            "price": 7.104,
            "diskType": "gp2",
            "diskSize": 10
        },
        "r6i.32xlarge": {
            "vcpus": 128,
            "memsize": 1048576,
            "price": 9.472,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.large": {
            "vcpus": 2,
            "memsize": 8192,
            "price": 0.112,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.xlarge": {
            "vcpus": 4,
            "memsize": 16384,
            "price": 0.224,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.2xlarge": {
            "vcpus": 8,
            "memsize": 32768,
            "price": 0.448,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.4xlarge": {
            "vcpus": 16,
            "memsize": 65536,
            "price": 0.896,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.8xlarge": {
            "vcpus": 32,
            "memsize": 131072,
            "price": 1.792,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.12xlarge": {
            "vcpus": 48,
            "memsize": 196608,
            "price": 2.688,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.16xlarge": {
            "vcpus": 64,
            "memsize": 262144,
            "price": 3.584,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.24xlarge": {
            "vcpus": 96,
            "memsize": 393216,
            "price": 5.376,
            "diskType": "gp2",
            "diskSize": 10
        },
        "m6i.32xlarge": {
            "vcpus": 128,
            "memsize": 524288,
            "price": 7.168,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.large": {
            "vcpus": 2,
            "memsize": 4096,
            "price": 0.101,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.xlarge": {
            "vcpus": 4,
            "memsize": 8192,
            "price": 0.202,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.2xlarge": {
            "vcpus": 8,
            "memsize": 16384,
            "price": 0.404,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.4xlarge": {
            "vcpus": 16,
            "memsize": 32768,
            "price": 0.808,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.8xlarge": {
            "vcpus": 32,
            "memsize": 65536,
            "price": 1.616,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.12xlarge": {
            "vcpus": 48,
            "memsize": 98304,
            "price": 2.424,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.16xlarge": {
            "vcpus": 64,
            "memsize": 131072,
            "price": 3.232,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.24xlarge": {
            "vcpus": 96,
            "memsize": 196608,
            "price": 4.848,
            "diskType": "gp2",
            "diskSize": 10
        },
        "c6i.32xlarge": {
            "vcpus": 128,
            "memsize": 262144,
            "price": 6.464,
            "diskType": "gp2",
            "diskSize": 10
        }
    },
    "sync-folder": {},
    "ssh-infos": {
        "wait-ssh-ready-seconds": 180,
        "user": "ubuntu",
        "ssh-private-key": "/etc/ssh/id_rsa"
    },
    "aws": {
        "aws-ca-k8s": {
            "accessKey": "12345678",
            "secretKey": "12345678",
            "token": "",
            "profile": "acme",
            "region": "eu-west-1",
            "keyName": "aws-k8s-key",
            "ami": "ami-12345678",
            "iam-role-arn": "arn:aws:iam::12345678:instance-profile/kubernetes-worker-profile",
            "timeout": 120,
            "tags": [
                {
                    "key": "CustomTag",
                    "value": "CustomValue"
                }
            ],
            "network": {
                "route53": "Z12345678",
                "privateZoneName": "acme.priv",
                "accessKey": "12345678",
                "secretKey": "12345678",
                "token": "",
                "profile": "acme",
                "region": "eu-west-1",
                "eni": [
                    {
                        "subnets": [
                            "subnet-12345678",
                            "subnet-45789",
                            "subnet-123458933"
                        ],
                        "securityGroup": "sg-123456789",
                        "publicIP": false
                    }
                ]
            }
        }
    }
}
```

# Unmaintened releases

All release before 1.26.11 are not maintened
