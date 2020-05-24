#!/bin/bash

make -e REGISTRY=fred78290 -e TAG=dev container

docker push fred78290/aws-autoscaler:dev