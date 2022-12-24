#/bin/bash
pushd $(dirname $0)

sudo rm -rf out vendor

make container

GOARCH=$(go env GOARCH)

./out/linux/$GOARCH/aws-autoscaler \
    --config=$HOME/Projects/autoscaled-masterkube-aws/config/aws-ca-k8s/config/kubernetes-aws-autoscaler.json \
    --save=$HOME/Projects/autoscaled-masterkube-aws/config/aws-ca-k8s/config/autoscaler-state.json \
    --log-level=info
