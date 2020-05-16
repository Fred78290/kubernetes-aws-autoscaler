#/bin/bash
pushd $(dirname $0)

make container

[ $(uname -s) = "Darwin" ] && GOOS=darwin || GOOS=linux

./out/aws-autoscaler-$GOOS-amd64 \
    --config=masterkube/config/kubernetes-aws-autoscaler.json \
    --save=masterkube/config/autoscaler-state.json \
    -v=9 \
    -logtostderr=true