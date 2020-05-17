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
    "timeout": 300,
    "ami": "${SEED_IMAGE}",
    "iam-role-arn": "${IAM_ROLE_ARN}",
    "instanceName": "test-kubernetes-aws-autoscaler",
    "instanceType": "t2.micro",
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
        "user": "~",
        "ssh-private-key": "~/.ssh/id_rsa"
    },
    "cloud-init": {
        "package_update": false,
        "package_upgrade": false
    }
}
EOF

echo "Run create instance"
go test --run Test_createInstance -race ./aws

echo "Run get instance"
go test --run Test_getInstanceID -race ./aws

echo "Run status instance"
go test --run Test_statusInstance -race ./aws

echo "Run wait ready"
go test --run Test_waitReadyInstance -race ./aws

#echo "Run power instance"
#go test --run Test_powerOffInstance -race ./aws

echo "Run shutdown instance"
go test --run Test_shutdownInstance -race ./aws

echo "Run test delete instance"
go test --run Test_deleteInstance -race ./aws
