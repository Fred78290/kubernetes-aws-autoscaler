#!/bin/bash
set -e

if [ -z $AWS_ACCESSKEY ] && [ -z $AWS_PROFILE ]; then
    exit 0
fi

go mod vendor

export Test_AuthMethodKey=NO
export Test_Sudo=NO
export Test_getInstanceID=YES
export Test_createInstance=YES
export Test_statusInstance=YES
export Test_waitForPowered=YES
export Test_waitForIP=YES
export Test_powerOnInstance=NO
export Test_powerOffInstance=YES
export Test_shutdownInstance=YES
export Test_deleteInstance=YES
export TEST_CONFIG=test.json

cat > ./aws/$TEST_CONFIG <<EOF
{
    "accessKey": "${AWS_ACCESSKEY}",
    "secretKey": "${AWS_SECRETKEY}",
    "token": "${AWS_TOKEN}",
    "profile": "${AWS_PROFILE}",
    "region" : "${AWS_REGION}",
    "keyName": "${SSH_KEYNAME}",
    "timeout": 60,
    "ami": "${SEED_IMAGE}",
    "iam-role-arn": "${IAM_ROLE_ARN}",
    "instanceName": "test-kubernetes-aws-autoscaler",
    "instanceType": "t3a.micro",
    "diskSize": 10,
    "tags": [
         {
            "key": "CustomTag",
            "value": "CustomValue"
        }
    ],
    "network": {
        "usePublicIP": true,
        "eni": [
            {
                "subnet": "${VPC_SUBNET_ID}",
                "securityGroup": "${VPC_SECURITY_GROUPID}",
                "publicIP": true
            }
        ]
    },
    "ssh": {
        "user": "$SEED_USER",
        "ssh-private-key": "~/.ssh/id_rsa"
    }
}
EOF

echo "Run create instance"
go test --run Test_createInstance -count 1 -race ./aws

echo "Run get instance"
go test --run Test_getInstanceID -count 1 -race ./aws

echo "Run status instance"
go test --run Test_statusInstance -count 1 -race ./aws

echo "Run wait for started"
go test --run Test_waitForPowered -count 1 -race ./aws

echo "Run wait for IP"
go test --run Test_waitForIP -count 1 -race ./aws

#echo "Run power instance"
#go test --run Test_powerOffInstance -count 1 -race ./aws

echo "Run shutdown instance"
go test --run Test_shutdownInstance -count 1 -race ./aws

echo "Run test delete instance"
go test --run Test_deleteInstance -count 1 -race ./aws
