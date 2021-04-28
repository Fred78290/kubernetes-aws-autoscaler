#!/bin/bash
sudo rm -rf out

VERSION=v1.21.0-dev
REGISTRY=fred78290

make -e REGISTRY=$REGISTRY -e TAG=$VERSION container-push-manifest
