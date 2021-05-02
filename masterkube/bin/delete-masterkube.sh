#!/bin/bash

CURDIR=$(dirname $0)
NODEGROUP_NAME="aws-ca-k8s"
MASTERKUBE=${NODEGROUP_NAME}-masterkube

echo "Delete masterkube previous instance"

pushd $CURDIR/../

if [ -f ./cluster/config ]; then
    for INSTANCE_ID in $(kubectl get node -o json --kubeconfig ./cluster/config | jq '.items| .[] | .metadata.annotations["cluster.autoscaler.nodegroup/instance-id"]' | tr -d '"')
    do
        echo "Delete Instance ID: $INSTANCE_ID"
        aws ec2 terminate-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids "${INSTANCE_ID}" &>/dev/null
    done
fi

./bin/kubeconfig-delete.sh $NODEGROUP_NAME &> /dev/null

if [ -f config/aws-autoscaler.pid ]; then
    kill $(cat config/aws-autoscaler.pid)
fi

find cluster ! -name '*.md' -type f -exec rm -f "{}" "+"
find config ! -name '*.md' -type f -exec rm -f "{}" "+"

if [ "$(uname -s)" == "Linux" ]; then
    sudo sed -i "/${MASTERKUBE}/d" /etc/hosts
else
    sudo sed -i'' "/${MASTERKUBE}/d" /etc/hosts
fi

popd