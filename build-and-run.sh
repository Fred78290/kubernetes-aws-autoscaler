#/bin/bash
pushd $(dirname $0)

make container

GOARCH=$(go env GOARCH)

./out/aws-autoscaler-$GOARCH \
    --config=~/Projects/autoscaled-masterkube-aws/config/aws-ca-k8s/kubernetes-aws-autoscaler.json \
    --save=~/Projects/autoscaled-masterkube-aws/config/aws-ca-k8s/autoscaler-state.json \
    --log-level=info
