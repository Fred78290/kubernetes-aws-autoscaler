#!/bin/sh

echo "Delete context: k8s-$1-masterkube-admin@$1"

kubectl config delete-context k8s-$1-masterkube-admin@$1

echo
echo "Delete context: $1"
kubectl config delete-cluster $1
