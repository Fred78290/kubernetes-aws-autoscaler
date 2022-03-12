[![Build Status](https://github.com/fred78290/kubernetes-aws-autoscaler/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/Fred78290/Fred78290_kubernetes-aws-autoscaler/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-aws-autoscaler&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-aws-autoscaler)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-aws-autoscaler/blob/master/LICENSE)

# kubernetes-aws-autoscaler

Kubernetes autoscaler for aws

## How it works

This tool will drive AWS to deploy EC2 instance at the demand. The cluster autoscaler deployment use an enhanced version of cluster-autoscaler. <https://github.com/Fred78290/autoscaler>. This version use grpc to communicate with the cloud provider hosted outside the pod. A docker image is available here <https://hub.docker.com/r/fred78290/cluster-autoscaler>

A sample of the cluster-autoscaler deployment is available at [examples/cluster-autoscaler.yaml](./examples/cluster-autoscaler.yaml). You must fill value between <>

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
