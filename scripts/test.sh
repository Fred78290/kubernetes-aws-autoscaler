#!/bin/bash
if [ -z "${GITHUB_RUN_ID}" ]; then
    echo "Can't run out of github action"
    exit 1
fi

cat > ./scripts/local.env <<EOF
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

export | grep GITHUB | sed 's/declare -x/export/g' >> ./scripts/local.env

make -e REGISTRY=fred78290 -e TAG=test-ci test-in-docker
