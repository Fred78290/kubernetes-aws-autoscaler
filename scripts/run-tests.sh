#!/bin/bash
CURDIR=$(dirname $0)
set -e

AWSDEFS=${CURDIR}/local.env
VERBOSE=-test.v

if [ ! -f "${AWSDEFS}" ]; then
  echo "File ${AWSDEFS} not found, exit test"
  exit 1
fi

source ${AWSDEFS}

if [ -z "${AWS_ACCESSKEY}" ] && [ -z "${AWS_PROFILE}" ]; then
    echo "Neither AWS_ACCESSKEY or AWS_PROFILE are defined, exit test"
    exit 1
fi

mkdir -p ${HOME}/.ssh

echo -n ${SSH_PRIVATEKEY} | base64 -d > ${HOME}/.ssh/test_rsa

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

export TEST_CONFIG=../test/test.json

cat > ./test/test.json <<EOF
{
    "accessKey": "${AWS_ACCESSKEY}",
    "secretKey": "${AWS_SECRETKEY}",
    "token": "${AWS_TOKEN}",
    "profile": "${AWS_PROFILE}",
    "region" : "${AWS_REGION}",
    "keyName": "${SSH_KEYNAME}",
    "timeout": 900,
    "ami": "${SEED_IMAGE}",
    "iam-role-arn": "${IAM_ROLE_ARN}",
    "instanceName": "test-kubernetes-aws-${GITHUB_RUN_ID}",
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
                "subnets": [
                  "${VPC_SUBNET_ID}"
                ],
                "securityGroup": "${VPC_SECURITY_GROUPID}",
                "publicIP": true
            }
        ]
    },
    "ssh": {
        "user": "${SEED_USER}",
        "ssh-private-key": "${HOME}/.ssh/test_rsa"
    }
}
EOF

cat > ./test/config.json <<EOF
{
  "use-external-etcd": false,
  "src-etcd-ssl-dir": "/etc/etcd/ssl",
  "dst-etcd-ssl-dir": "/etc/kubernetes/pki/etcd",
  "kubernetes-pki-srcdir": "/etc/kubernetes/pki",
  "kubernetes-pki-dstdir": "/etc/kubernetes/pki",
  "network": "unix",
  "listen": "/var/run/cluster-autoscaler/aws.sock",
  "secret": "aws",
  "minNode": 0,
  "maxNode": 9,
  "maxPods": 17,
  "node-name-prefix": "autoscaled",
  "managed-name-prefix": "managed",
  "controlplane-name-prefix": "master",
  "nodePrice": 0,
  "podPrice": 0,
  "image": "${SEED_IMAGE}",
  "cloud-provider": "external",
  "optionals": {
    "pricing": false,
    "getAvailableMachineTypes": false,
    "newNodeGroup": false,
    "templateNodeInfo": false,
    "createNodeGroup": false,
    "deleteNodeGroup": false
  },
  "kubeadm": {
    "address": "172.30.127.112:6443",
    "token": "XXX.YYYYYY",
    "ca": "sha256:23eaa8c4dfb176993008bd9443cd1346ba2be0737e53ce0367d09456abb4ef05",
    "extras-args": [
      "--ignore-preflight-errors=All"
    ]
  },
  "default-machine": "t3a.micro",
  "machines": {
    "t3a.nano": {
      "price": 0.0051,
      "memsize": 512,
      "vcpus": 2,
      "diskType": "gp2",
      "diskSize": 10
    },
    "t3a.micro": {
      "price": 0.0102,
      "memsize": 1024,
      "vcpus": 2,
      "diskType": "gp2",
      "diskSize": 10
    },
    "t3a.small": {
      "price": 0.0204,
      "memsize": 2048,
      "vcpus": 2,
      "diskType": "gp2",
      "diskSize": 10
    },
    "t3a.medium": {
      "price": 0.0408,
      "memsize": 4096,
      "vcpus": 2,
      "diskType": "gp2",
      "diskSize": 10
    },
    "t3a.large": {
      "price": 0.0816,
      "memsize": 8192,
      "vcpus": 2,
      "diskType": "gp2",
      "diskSize": 10
    },
    "t3a.xlarge": {
      "price": 0.1632,
      "memsize": 16384,
      "vcpus": 4,
      "diskType": "gp2",
      "diskSize": 10
    },
    "t3a.2xlarge": {
      "price": 0.3264,
      "memsize": 32768,
      "vcpus": 8,
      "diskType": "gp2",
      "diskSize": 10
    }
  },
  "sync-folder": {},
  "ssh-infos": {
    "user": "${SEED_USER}",
    "ssh-private-key": "${HOME}/.ssh/test_rsa"
  },
  "aws": {
    "aws-${GITHUB_RUN_ID}": {
      "profile": "${AWS_PROFILE}",
      "region": "${AWS_REGION}",
      "accessKey": "${AWS_ACCESSKEY}",
      "secretKey": "${AWS_SECRETKEY}",
      "token": "${AWS_TOKEN}",
      "keyName": "${SSH_KEYNAME}",
      "ami": "${SEED_IMAGE}",
      "iam-role-arn": "${IAM_ROLE_ARN}",
      "timeout": 900,
      "tags": [
        {
          "key": "CustomTag",
          "value": "CustomValue"
        }
      ],
      "network": {
        "route53": "${ROUTE53_ZONEID}",
        "privateZoneName": "${PRIVATE_DOMAIN_NAME}",
        "eni": [
          {
            "subnets": [
                "${VPC_SUBNET_ID}"
            ],
            "securityGroup": "${VPC_SECURITY_GROUPID}",
            "publicIP": true
          }
        ]
      }
    }
  }
}
EOF

go clean -testcache
go mod vendor

# Run this test only on github action
if [ ! -z "${GITHUB_REF}" ]; then
  echo "Run create instance"
  go test $VERBOSE --run Test_createInstance -timeout 60s -count 1 -race ./aws

  echo "Run get instance"
  go test $VERBOSE --run Test_getInstanceID -timeout 60s -count 1 -race ./aws

  echo "Run status instance"
  go test $VERBOSE --run Test_statusInstance -timeout 60s -count 1 -race ./aws

  echo "Run wait for started"
  go test $VERBOSE --run Test_waitForPowered -timeout 60s -count 1 -race ./aws

  echo "Run wait for IP"
  go test $VERBOSE --run Test_waitForIP -timeout 120s -count 1 -race ./aws

  #echo "Run power instance"
  #go test $VERBOSE --run Test_powerOffInstance -count 1 -race ./aws

  echo "Run shutdown instance"
  go test $VERBOSE --run Test_shutdownInstance -timeout 600s -count 1 -race ./aws

  echo "Run test delete instance"
  go test $VERBOSE --run Test_deleteInstance -timeout 60s -count 1 -race ./aws
fi

export TEST_CONFIG=../test/config.json

echo "Run server test"

export TestServer=YES
export TestServer_NodeGroups=YES
export TestServer_NodeGroupForNode=YES
export TestServer_HasInstance=YES
export TestServer_Pricing=YES
export TestServer_GetAvailableMachineTypes=YES
export TestServer_NewNodeGroup=YES
export TestServer_GetResourceLimiter=YES
export TestServer_Cleanup=YES
export TestServer_Refresh=YES
export TestServer_TargetSize=YES
export TestServer_IncreaseSize=YES
export TestServer_DecreaseTargetSize=YES
export TestServer_DeleteNodes=YES
export TestServer_Id=YES
export TestServer_Debug=YES
export TestServer_Nodes=YES
export TestServer_TemplateNodeInfo=YES
export TestServer_Exist=YES
export TestServer_Create=YES
export TestServer_Delete=YES
export TestServer_Autoprovisioned=YES
export TestServer_Belongs=YES
export TestServer_NodePrice=YES
export TestServer_PodPrice=YES

go test $VERBOSE --test.short -timeout 1200s -race ./server -run Test_Server

echo "Run nodegroup test"

export TestNodegroup=YES
export TestNodeGroup_launchVM=YES
export TestNodeGroup_stopVM=YES
export TestNodeGroup_startVM=YES
export TestNodeGroup_statusVM=YES
export TestNodeGroup_deleteVM=YES
export TestNodeGroupGroup_addNode=YES
export TestNodeGroupGroup_deleteNode=YES
export TestNodeGroupGroup_deleteNodeGroup=YES

go test $VERBOSE --test.short -timeout 1200s -race ./server -run Test_Nodegroup
