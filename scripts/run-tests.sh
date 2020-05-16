#!/bin/bash
set -e

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


cat > ./aws/test.json <<EOF
{
    "accessKey": "${AWS_ACCESSKEY}",
    "secretKey": "${AWS_SECRETKEY}",
    "token": "${AWS_TOKEN}",
    "profile": "${AWS_PROFILE}",
    "region" : "${AWS_REGION}",
    "keyName": "${SSH_KEYNAME}",
    "timeout": 300,
    "tags": [
        {
            "key": "NodeGroup",
            "value": "aws-ca-k8s"
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
    },
    "instanceName": "test-kubernetes-aws-autoscaler"
}
EOF

echo "Run test"
go test --run Test_createInstance
go test --run Test_getInstanceID
go test --run Test_statusInstance
go test --run Test_powerOffInstance
go test --run Test_shutdownInstance
go test --run Test_deleteInstance
