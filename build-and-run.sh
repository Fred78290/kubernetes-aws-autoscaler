#/bin/bash
pushd $(dirname $0)

make container

GOARCH=$(go env GOARCH)

./out/aws-autoscaler-$GOARCH \
    --config=masterkube/config/kubernetes-aws-autoscaler.json \
    --save=masterkube/config/autoscaler-state.json \
    --log-level=info
