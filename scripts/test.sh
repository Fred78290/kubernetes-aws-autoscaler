#!/bin/bash
if [ -z "${GITHUB_RUN_ID}" ]; then
    echo "Can't run out of github action"
    exit 1
fi

export | sed 's/declare -x/export /g' > ./scripts/local.env

make -e REGISTRY=fred78290 -e TAG=test-ci test-in-docker
