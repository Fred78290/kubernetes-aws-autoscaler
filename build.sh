#!/bin/bash
sudo rm -rf out

VERSION=v1.23.8
REGISTRY=fred78290

make -e REGISTRY=$REGISTRY -e TAG=$VERSION container-push-manifest
