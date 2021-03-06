[![Build Status](https://github.com/fred78290/kubernetes-aws-autoscaler/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/Fred78290/Fred78290_kubernetes-aws-autoscaler/actions)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Fred78290_kubernetes-aws-autoscaler&metric=alert_status)](https://sonarcloud.io/dashboard?id=Fred78290_kubernetes-aws-autoscaler)
[![Licence](https://img.shields.io/hexpm/l/plug.svg)](https://github.com/Fred78290/kubernetes-aws-autoscaler/blob/master/LICENSE)


# kubernetes-aws-autoscaler

Kubernetes autoscaler for aws

## How it works

This tool will drive AWS to deploy EC2 instance at the demand. The cluster autoscaler deployment use an enhanced version of cluster-autoscaler. https://github.com/Fred78290/autoscaler. This version use grpc to communicate with the cloud provider hosted outside the pod. A docker image is available here https://hub.docker.com/r/fred78290/cluster-autoscaler

A sample of the cluster-autoscaler deployment is available at [examples/cluster-autoscaler.yaml](./examples/cluster-autoscaler.yaml). You must fill value between <>

Before you must deploy your kubernetes cluster on vSphere. You can do it from scrash or you can use the script [masterkube/bin/create-masterkube.sh](./masterkube/bin/create-masterkube.sh) to create a simple VM hosting the kubernetes master node.

## Commandline arguments

| Parameter | Description |
| --- | --- |
| `version` | Print the version and exit  |
| `save`  | Tell the tool to save state in this file  |
| `config`  |The the tool to use config file |

## Build

The build process use make file. The simplest way to build is `make container`
