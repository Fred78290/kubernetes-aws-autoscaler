#!/bin/bash
cat > ./test/local.env <<EOF
export SEED_IMAGE=$SEED_IMAGE
export IAM_ROLE_ARN=$IAM_ROLE_ARN
export SSH_KEYNAME=$SSH_KEYNAME
export SSH_PRIVATEKEY=$SSH_PRIVATEKEY
export SEED_IMAGE=$SEED_IMAGE
export SEED_USER=$SEED_USER

export VPC_SECURITY_GROUPID=$VPC_SECURITY_GROUPID
export VPC_SUBNET_ID=$VPC_SUBNET_ID
export ROUTE53_ZONEID=$ROUTE53_ZONEID

export AWS_PROFILE=$AWS_PROFILE
export AWS_REGION=$AWS_REGION
export AWS_ACCESSKEY=$AWS_ACCESSKEY
export AWS_SECRETKEY=$AWS_SECRETKEY

export PRIVATE_DOMAIN_NAME=$PRIVATE_DOMAIN_NAME
export PUBLIC_DOMAIN_NAME=$PUBLIC_DOMAIN_NAME
EOF

make -e REGISTRY=$REGISTRY -e TAG=test-ci test-in-docker
