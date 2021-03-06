#/bin/bash

# This script create every thing to deploy a simple kubernetes autoscaled cluster with aws.
# It will generate:
# Custom AMI image with every thing for kubernetes
# Config file to deploy the cluster autoscaler.

set -e

CURDIR=$(dirname $0)

export SCHEME="aws"
export NODEGROUP_NAME="aws-ca-k8s"
export MASTERKUBE="${NODEGROUP_NAME}-masterkube"
export PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
export SSH_PRIVATE_KEY=~/.ssh/id_rsa
export SSH_PUBLIC_KEY="${SSH_PRIVATE_KEY}.pub"
export KUBERNETES_VERSION=v1.21.2
export KUBECONFIG=${HOME}/.kube/config
export ROOT_IMG_NAME=focal-kubernetes
export CNI_VERSION=v0.6.0
export CNI_PLUGIN_VERSION=v0.9.1
export CNI_PLUGIN=aws
export CLOUD_PROVIDER=aws
export MINNODES=0
export MAXNODES=5
export MAXTOTALNODES=${MAXNODES}
export CORESTOTAL="0:16"
export MEMORYTOTAL="0:24"
export MAXAUTOPROVISIONNEDNODEGROUPCOUNT="1"
export SCALEDOWNENABLED="true"
export SCALEDOWNDELAYAFTERADD="1m"
export SCALEDOWNDELAYAFTERDELETE="1m"
export SCALEDOWNDELAYAFTERFAILURE="1m"
export SCALEDOWNUNEEDEDTIME="1m"
export SCALEDOWNUNREADYTIME="1m"
export DEFAULT_MACHINE="t3a.medium"
export UNREMOVABLENODERECHECKTIMEOUT="1m"
export OSDISTRO=$(uname -s)
export TRANSPORT="tcp"
export SSH_KEYNAME="aws-k8s-key"
export VOLUME_SIZE=10
export MAX_PODS=110
export MASTER_PROFILE_NAME="kubernetes-master-profile"
export WORKER_PROFILE_NAME="kubernetes-worker-profile"
export TARGET_IMAGE="${ROOT_IMG_NAME}-cni-${CNI_PLUGIN}-${KUBERNETES_VERSION}-${SEED_ARCH}"
export REGISTRY=fred78290

# aws region eu-west1
export SEED_ARCH=amd64
export SEED_USER=ubuntu
export SEED_IMAGE_AMD64="ami-08bac620dc84221eb"
export SEED_IMAGE_ARM64="ami-09e0d6fdf60750e33"
# defined in private aws.defs
export MASTER_INSTANCE_PROFILE_ARN="<to be filled>"
export WORKER_INSTANCE_PROFILE_ARN="<to be filled>"
export VPC_MASTER_SUBNET_ID="<to be filled>"
export VPC_MASTER_SECURITY_GROUPID="<to be filled>"
export VPC_WORKER_SUBNET_ID="<to be filled>"
export VPC_WORKER_SECURITY_GROUPID="<to be filled>"
export ACM_CERTIFICATE_NAME="<to be filled>"
export ROUTE53_ZONEID="<to be filled>"

export VPC_MASTER_USE_PUBLICIP=true
export VPC_WORKER_USE_PUBLICIP=false

export LAUNCH_CA=YES
export PRIVATE_DOMAIN_NAME=

# import hidded definitions
source ${CURDIR}/aws.defs

SSH_OPTIONS="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

if [ "${OSDISTRO}" == "Linux" ]; then
    TZ=$(cat /etc/timezone)
    BASE64="base64 -w 0"
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
    BASE64="base64"
fi

TEMP=$(getopt -o p:r:k:n:p:s:t: --long arch:,cloud-provider:,max-pods:,profile:,region:,node-group:,target-image:,seed-image:,seed-user:,vpc-id:,public-subnet-id:,public-sg-id:,private-subnet-id:,private-sg-id:,transport:,ssh-private-key:,cni-version:,cni-plugin-version:,kubernetes-version:,max-nodes-total:,cores-total:,memory-total:,max-autoprovisioned-node-group-count:,scale-down-enabled:,scale-down-delay-after-add:,scale-down-delay-after-delete:,scale-down-delay-after-failure:,scale-down-unneeded-time:,scale-down-unready-time:,unremovable-node-recheck-timeout: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -p|--profile)
        AWS_PROFILE="$2"
        shift 2
        ;;
    -r|--region)
        AWS_REGION="$2"
        shift 2
        ;;

    --max-pods)
        MAX_PODS=$2
        shift 2
        ;;

    --node-group)
        NODEGROUP_NAME="$2"
        MASTERKUBE="${NODEGROUP_NAME}-masterkube"
        PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
        shift 2
        ;;

    --target-image)
        ROOT_IMG_NAME="$2"
        shift 2
        ;;

    --arch)
        SEED_ARCH=$2
        shift 2
        ;;

    --seed-image)
        SEED_IMAGE="$2"
        shift 2
        ;;

    --seed-user)
        SEED_USER="$2"
        shift 2
        ;;

    --public-subnet-id)
        VPC_MASTER_SUBNET_ID="$2"
        shift 2
        ;;

    --public-sg-id)
        VPC_MASTER_SECURITY_GROUPID="$2"
        shift 2
        ;;

    --private-subnet-id)
        VPC_WORKER_SUBNET_ID="$2"
        shift 2
        ;;

    --private-sg-id)
        VPC_WORKER_SECURITY_GROUPID="$2"
        shift 2
        ;;

    -d | --default-machine)
        DEFAULT_MACHINE="$2"
        shift 2
        ;;
    -s | --ssh-private-key)
        SSH_PRIVATE_KEY=$2
        shift 2
        ;;
    -n | --cni-version)
        CNI_VERSION="$2"
        shift 2
        ;;
    -c | --cni-plugin-version)
        CNI_PLUGIN_VERSION="$2"
        shift 2
        ;;
    -t | --transport)
        TRANSPORT="$2"
        shift 2
        ;;
    -k | --kubernetes-version)
        KUBERNETES_VERSION="$2"
        shift 2
        ;;

    # Same argument as cluster-autoscaler
    --max-nodes-total)
        MAXTOTALNODES="$2"
        shift 2
        ;;
    --cores-total)
        CORESTOTAL="$2"
        shift 2
        ;;
    --memory-total)
        MEMORYTOTAL="$2"
        shift 2
        ;;
    --max-autoprovisioned-node-group-count)
        MAXAUTOPROVISIONNEDNODEGROUPCOUNT="$2"
        shift 2
        ;;
    --scale-down-enabled)
        SCALEDOWNENABLED="$2"
        shift 2
        ;;
    --scale-down-delay-after-add)
        SCALEDOWNDELAYAFTERADD="$2"
        shift 2
        ;;
    --scale-down-delay-after-delete)
        SCALEDOWNDELAYAFTERDELETE="$2"
        shift 2
        ;;
    --scale-down-delay-after-failure)
        SCALEDOWNDELAYAFTERFAILURE="$2"
        shift 2
        ;;
    --scale-down-unneeded-time)
        SCALEDOWNUNEEDEDTIME="$2"
        shift 2
        ;;
    --scale-down-unready-time)
        SCALEDOWNUNREADYTIME="$2"
        shift 2
        ;;
    --unremovable-node-recheck-timeout)
        UNREMOVABLENODERECHECKTIMEOUT="$2"
        shift 2
        ;;
    --)
        shift
        break
        ;;
    *)
        echo "$1 - Internal error!"
        exit 1
        ;;
    esac
done

if [ "$SEED_ARCH" == "amd64" ]; then
    export SEED_IMAGE=$SEED_IMAGE_AMD64
    export DEFAULT_MACHINE="t3a.medium"
    export MACHINES_TYPES=$(cat<<EOF
    {
        "t3a.nano": {
            "price": 0.0051,
            "memsize": 512,
            "vcpus": 2,
            "disksize": 10
        },
        "t3a.micro": {
            "price": 0.0102,
            "memsize": 1024,
            "vcpus": 2,
            "disksize": 10
        },
        "t3a.small": {
            "price": 0.0204,
            "memsize": 2048,
            "vcpus": 2,
            "disksize": 10
        },
        "t3a.medium": {
            "price": 0.0408,
            "memsize": 4096,
            "vcpus": 2,
            "disksize": 10
        },
        "t3a.large": {
            "price": 0.0816,
            "memsize": 8192,
            "vcpus": 2,
            "disksize": 10
        },
        "t3a.xlarge": {
            "price": 0.1632,
            "memsize": 16384,
            "vcpus": 4,
            "disksize": 10
        },
        "t3a.2xlarge": {
            "price": 0.3264,
            "memsize": 32768,
            "vcpus": 8,
            "disksize": 10
        }
    }
EOF
    )
elif [ "$SEED_ARCH" == "arm64" ]; then
    export SEED_IMAGE=$SEED_IMAGE_ARM64
    export DEFAULT_MACHINE="t4g.medium"
    export MACHINES_TYPES=$(cat<<EOF
    {
        "t4g.nano": {
            "price": 0.0046,
            "memsize": 512,
            "vcpus": 2,
            "disksize": 10
        },
        "t4g.micro": {
            "price": 0.096,
            "memsize": 1024,
            "vcpus": 2,
            "disksize": 10
        },
        "t4g.small": {
            "price": 0.0184,
            "memsize": 2048,
            "vcpus": 2,
            "disksize": 10
        },
        "t4g.medium": {
            "price": 0.0368,
            "memsize": 4096,
            "vcpus": 2,
            "disksize": 10
        },
        "t4g.large": {
            "price": 0.0736,
            "memsize": 8192,
            "vcpus": 2,
            "disksize": 10
        },
        "t4g.xlarge": {
            "price": 0.1472,
            "memsize": 16384,
            "vcpus": 4,
            "disksize": 10
        },
        "t4g.2xlarge": {
            "price": 0.2944,
            "memsize": 32768,
            "vcpus": 8,
            "disksize": 10
        }
    }
EOF
    )
else
    echo "Unsupported architecture: $SEED_ARCH"
    exit -1
fi

pushd ${CURDIR}/../

# If we use AWS CNI, install eni-max-pods.txt definition file
if [ $CNI_PLUGIN = "aws" ]; then
    AWS_MAX_PODS=$(curl -s "https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/files/eni-max-pods.txt" | grep ^${DEFAULT_MACHINE} | awk '{print $2}')

    if [ -z "$AWS_MAX_PODS" ]; then
        echo "No entry for ${DEFAULT_MACHINE} in eni-max-pods.txt. Not setting ${MAX_PODS} max pods for kubelet"
    else
        MAX_PODS=${AWS_MAX_PODS}
    fi
fi

# If no master instance profile defined, use the default
if [ -z $MASTER_INSTANCE_PROFILE_ARN ]; then
    MASTER_INSTANCE_PROFILE_ARN=$(aws iam get-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${MASTER_PROFILE_NAME} 2> /dev/null | jq '.InstanceProfile.Arn' | tr -d '"')

    # If not found, create it
    if [ -z $MASTER_INSTANCE_PROFILE_ARN ]; then
        aws iam create-role --profile ${AWS_PROFILE} --region ${AWS_REGION} --role-name ${MASTER_PROFILE_NAME} --assume-role-policy-document file://templates/profile/master/trusted.json &> /dev/null
        aws iam put-role-policy --profile ${AWS_PROFILE} --region ${AWS_REGION} --role-name ${MASTER_PROFILE_NAME} --policy-name kubernetes-master-permissions --policy-document file://templates/profile/master/permissions.json &> /dev/null
        aws iam create-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${MASTER_PROFILE_NAME} &> /dev/null
        aws iam add-role-to-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${MASTER_PROFILE_NAME} --role-name ${MASTER_PROFILE_NAME} &> /dev/null

        MASTER_INSTANCE_PROFILE_ARN=$(aws iam get-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${MASTER_PROFILE_NAME} | jq '.InstanceProfile.Arn' | tr -d '"')
    fi
fi

# If no worker instance profile defined, use the default
if [ -z $WORKER_INSTANCE_PROFILE_ARN ]; then
    WORKER_INSTANCE_PROFILE_ARN=$(aws iam get-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${WORKER_PROFILE_NAME} 2> /dev/null | jq '.InstanceProfile.Arn' | tr -d '"')

    # If not found, create it
    if [ -z $WORKER_INSTANCE_PROFILE_ARN ]; then
        aws iam create-role --profile ${AWS_PROFILE} --region ${AWS_REGION} --role-name ${WORKER_PROFILE_NAME} --assume-role-policy-document file://templates/profile/worker/trusted.json &> /dev/null
        aws iam put-role-policy --profile ${AWS_PROFILE} --region ${AWS_REGION} --role-name ${WORKER_PROFILE_NAME} --policy-name kubernetes-worker-permissions --policy-document file://templates/profile/worker/permissions.json &> /dev/null
        aws iam create-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${WORKER_PROFILE_NAME} &> /dev/null
        aws iam add-role-to-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${WORKER_PROFILE_NAME} --role-name ${WORKER_PROFILE_NAME} &> /dev/null

        WORKER_INSTANCE_PROFILE_ARN=$(aws iam get-instance-profile --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-profile-name ${WORKER_PROFILE_NAME} | jq '.InstanceProfile.Arn' | tr -d '"')
    fi
fi

# Tag VPC & Subnet
TAGGED=$(aws ec2 describe-subnets --profile ${AWS_PROFILE} --region ${AWS_REGION} --filters "Name=subnet-id,Values=${VPC_MASTER_SUBNET_ID}" | jq ".Subnets[].Tags[]|select(.Key == \"kubernetes.io/cluster/${NODEGROUP_NAME}\")|.Value" | tr -d '"')
if [ -z $TAGGED ]; then
    aws ec2 create-tags --profile ${AWS_PROFILE} --region ${AWS_REGION} --resources ${VPC_MASTER_SUBNET_ID} --tags "Key=kubernetes.io/cluster/${NODEGROUP_NAME},Value=owned" 2> /dev/null
fi

# Tag VPC & Subnet
TAGGED=$(aws ec2 describe-subnets --profile ${AWS_PROFILE} --region ${AWS_REGION} --filters "Name=subnet-id,Values=$VPC_WORKER_SUBNET_ID" | jq ".Subnets[].Tags[]|select(.Key == \"kubernetes.io/cluster/${NODEGROUP_NAME}\")|.Value" | tr -d '"')
if [ -z $TAGGED ]; then
    aws ec2 create-tags --profile ${AWS_PROFILE} --region ${AWS_REGION} --resources ${VPC_WORKER_SUBNET_ID} --tags "Key=kubernetes.io/cluster/${NODEGROUP_NAME},Value=owned" 2> /dev/null
fi

TARGET_IMAGE="${ROOT_IMG_NAME}-cni-${CNI_PLUGIN}-${KUBERNETES_VERSION}-${SEED_ARCH}"
KEYEXISTS=$(aws ec2 describe-key-pairs --profile ${AWS_PROFILE} --region ${AWS_REGION} --key-names "${SSH_KEYNAME}" | jq  '.KeyPairs[].KeyName' | tr -d '"')
ECR_PASSWORD=$(aws ecr get-login-password  --profile ${AWS_PROFILE} --region us-west-2)

SSH_KEY_FNAME=$(basename ${SSH_PRIVATE_KEY})
SSH_PUBLIC_KEY="${SSH_PRIVATE_KEY}.pub"

if [ -z ${KEYEXISTS} ]; then
    echo "SSH Public key doesn't exist"
    if [ ! -f ${SSH_PUBLIC_KEY} ]; then
        echo "${SSH_PUBLIC_KEY} doesn't exists. FATAL"

        exit -1
    fi
    aws ec2 import-key-pair --profile ${AWS_PROFILE} --region ${AWS_REGION} --key-name ${SSH_KEYNAME} --public-key-material "file://${SSH_PUBLIC_KEY}"
else
    echo "SSH Public key already exists"
fi

# GRPC network endpoint
if [ "${LAUNCH_CA}" != "YES" ]; then
    SSH_PRIVATE_KEY_LOCAL="${SSH_PRIVATE_KEY}"

    if [ "${TRANSPORT}" == "unix" ]; then
        LISTEN="/var/run/cluster-autoscaler/aws.sock"
        CONNECTTO="unix:/var/run/cluster-autoscaler/aws.sock"
    elif [ "${TRANSPORT}" == "tcp" ]; then
        if [ "${OSDISTRO}" == "Linux" ]; then
            NET_IF=$(ip route get 1 | awk '{print $5;exit}')
            IPADDR=$(ip addr show ${NET_IF} | grep -m 1 "inet\s" | tr '/' ' ' | awk '{print $2}')
        else
            NET_IF=$(route get 1 | grep -m 1 interface | awk '{print $2}')
            IPADDR=$(ifconfig ${NET_IF} | grep -m 1 "inet\s" | sed -n 1p | awk '{print $2}')
        fi

        LISTEN="${IPADDR}:5200"
        CONNECTTO="${IPADDR}:5200"
    else
        echo "Unknown transport: ${TRANSPORT}, should be unix or tcp"
        exit -1
    fi
else
    SSH_PRIVATE_KEY_LOCAL="/root/.ssh/id_rsa"
    TRANSPORT=unix
    LISTEN="/var/run/cluster-autoscaler/aws.sock"
    CONNECTTO="unix:/var/run/cluster-autoscaler/aws.sock"
fi

echo "Transport set to:${TRANSPORT}, listen endpoint at ${LISTEN}"


[ -d config ] || mkdir -p config
[ -d cluster ] || mkdir -p cluster

export PATH=./bin:${PATH}

# If CERT doesn't exist, create one autosigned
if [ ! -f ./etc/ssl/privkey.pem ]; then
    mkdir -p ./etc/ssl/
    openssl genrsa 2048 >./etc/ssl/privkey.pem
    openssl req -new -x509 -nodes -sha1 -days 3650 -key ./etc/ssl/privkey.pem >./etc/ssl/cert.pem
    cat ./etc/ssl/cert.pem ./etc/ssl/privkey.pem >./etc/ssl/fullchain.pem
    chmod 644 ./etc/ssl/*
fi

export TARGET_IMAGE_AMI=$(aws ec2 describe-images --profile ${AWS_PROFILE} --region ${AWS_REGION} --filters "Name=name,Values=${TARGET_IMAGE}" | jq '.Images[0].ImageId' | tr -d '"' | sed -e 's/null//g')

# Extract the domain name from CERT
export DOMAIN_NAME=$(openssl x509 -noout -subject -in ./etc/ssl/cert.pem | awk -F= '{print $NF}' | sed -e 's/^[ \t]*//' | sed 's/\*\.//g')

export ACM_CERTIFICATE_ARN=$(aws acm list-certificates --profile ${AWS_PROFILE} --region ${AWS_REGION} | jq --arg DOMAIN_NAME ${DOMAIN_NAME} '.CertificateSummaryList[]|select(.DomainName == $DOMAIN_NAME)|.CertificateArn' | tr -d '"' | sed -e 's/null//g')

if [ -z $ACM_CERTIFICATE_ARN ]; then
    aws acm import-certificate --profile ${AWS_PROFILE} --region ${AWS_REGION} --tags "Key=Name,Value=${ACM_CERTIFICATE_NAME}" \
        --certificate file://etc/ssl/cert.pem --certificate-chain file://etc/ssl/chain.pem --private-key file://etc/ssl/privkey.pem
    export ACM_CERTIFICATE_ARN=$(aws acm list-certificates --profile ${AWS_PROFILE} --region ${AWS_REGION} \
        | jq --arg DOMAIN_NAME ${DOMAIN_NAME} '.CertificateSummaryList[]|select(.DomainName == $DOMAIN_NAME)|.CertificateArn' | tr -d '"' | sed -e 's/null//g')
fi               

# If the VM template doesn't exists, build it from scrash
if [ -z "${TARGET_IMAGE_AMI}" ]; then
    echo "Create aws preconfigured image ${TARGET_IMAGE}"

    ./bin/create-image.sh \
        --profile="${AWS_PROFILE}" \
        --region="${AWS_REGION}" \
        --cni-version="${CNI_VERSION}" \
        --cni-plugin-version="${CNI_PLUGIN_VERSION}" \
        --cni-plugin="${CNI_PLUGIN}" \
        --ecr-password="${ECR_PASSWORD}" \
        --custom-image="${TARGET_IMAGE}" \
        --kubernetes-version="${KUBERNETES_VERSION}" \
        --arch="${SEED_ARCH}" \
        --ami="${SEED_IMAGE}" \
        --user="${SEED_USER}" \
        --ssh-key-name="${SSH_KEYNAME}" \
        --subnet-id="${VPC_MASTER_SUBNET_ID}" \
        --sg-id="${VPC_MASTER_SECURITY_GROUPID}" \
        --use-public-ip="${VPC_MASTER_USE_PUBLICIP}"
fi

export TARGET_IMAGE_AMI=$(aws ec2 describe-images --profile ${AWS_PROFILE} --region ${AWS_REGION} --filters "Name=name,Values=${TARGET_IMAGE}" | jq '.Images[0].ImageId' | tr -d '"' | sed -e 's/null//g')

if [ -d ${TARGET_IMAGE_AMI} ]; then
    echo "AMI ${TARGET_IMAGE} not found"
    exit -1
fi

# Grab private domain name
if [ ! -z $ROUTE53_ZONEID ]; then
    PRIVATE_DOMAIN_NAME=$(aws route53 get-hosted-zone --id  ${ROUTE53_ZONEID} --profile ${AWS_PROFILE} --region ${AWS_REGION} | jq .HostedZone.Name | tr -d '"' | sed -e 's/null//g')
    PRIVATE_DOMAIN_NAME=${PRIVATE_DOMAIN_NAME::-1}
fi

# Delete previous exixting version
delete-masterkube.sh

echo "Launch custom ${MASTERKUBE} instance with ${TARGET_IMAGE}"

# Record Masterkube in Route53 DNS
if [ -z $ROUTE53_ZONEID ]; then
    FQHOSTNAME=${MASTERKUBE}
else
    FQHOSTNAME=${MASTERKUBE}.${PRIVATE_DOMAIN_NAME}
fi

# Cloud init user-data
echo "#cloud-config" >./config/userdata.yaml

cat > ./config/userdata.yaml <<EOF
#cloud-config
runcmd:
- echo "Create ${MASTERKUBE}" > /var/log/masterkube.log
- hostnamectl set-hostname "${FQHOSTNAME}"
EOF

cat > ./config/mapping.json <<EOF
[
    {
        "DeviceName": "/dev/sda1",
        "Ebs": {
            "DeleteOnTermination": true,
            "VolumeType": "standard",
            "VolumeSize": ${VOLUME_SIZE},
            "Encrypted": false
        }
    }
]
EOF

if [ "${VPC_MASTER_USE_PUBLICIP}" == "true" ]; then
    PUBLIC_IP_OPTIONS=--associate-public-ip-address
else
    PUBLIC_IP_OPTIONS=--no-associate-public-ip-address
fi

echo "Clone ${TARGET_IMAGE} to ${MASTERKUBE}"
LAUNCHED_INSTANCE=$(aws ec2 run-instances \
    --profile "${AWS_PROFILE}" \
    --region "${AWS_REGION}" \
    --image-id "${TARGET_IMAGE_AMI}" \
    --count 1  \
    --instance-type "${DEFAULT_MACHINE}" \
    --key-name "${SSH_KEYNAME}" \
    --subnet-id "${VPC_MASTER_SUBNET_ID}" \
    --security-group-ids "${VPC_MASTER_SECURITY_GROUPID}" \
    --user-data "file://config/userdata.yaml" \
    --iam-instance-profile "Arn=${MASTER_INSTANCE_PROFILE_ARN}" \
    --block-device-mappings "file://config/mapping.json" \
    --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=${MASTERKUBE}},{Key=NodeGroup,Value=${NODEGROUP_NAME}},{Key=kubernetes.io/cluster/${NODEGROUP_NAME},Value=owned},{Key=KubernetesCluster,Value=${NODEGROUP_NAME}}]" \
    ${PUBLIC_IP_OPTIONS})

LAUNCHED_ID=$(echo ${LAUNCHED_INSTANCE} | jq '.Instances[0].InstanceId' | tr -d '"' | sed -e 's/null//g')

if [ -z ${LAUNCHED_ID} ]; then
    echo "Something goes wrong when launching ${MASTERKUBE}"
    exit -1
fi

echo "Launched ${MASTERKUBE} with ID=${LAUNCHED_ID}"

echo -n "Wait for ${MASTERKUBE} instanceID ${LAUNCHED_ID} to boot"

while [ ! $(aws ec2  describe-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids "${LAUNCHED_ID}" | jq .Reservations[0].Instances[0].State.Code) -eq 16 ];
do
    echo -n "."
    sleep 1
done

echo

LAUNCHED_INSTANCE=$(aws ec2  describe-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids ${LAUNCHED_ID} | jq .Reservations[0].Instances[0])

if [ "${VPC_MASTER_USE_PUBLICIP}" == "true" ]; then
    export IPADDR=$(echo ${LAUNCHED_INSTANCE} | jq '.PublicIpAddress' | tr -d '"' | sed -e 's/null//g')
    CERT_EXTRA_SANS="--cert-extra-sans ${IPADDR},${MASTERKUBE}.${DOMAIN_NAME},masterkube-aws.${DOMAIN_NAME},masterkube-aws-dashboard.${DOMAIN_NAME}"
    IP_TYPE="public"
else
    export IPADDR=$(echo ${LAUNCHED_INSTANCE} | jq '.PrivateIpAddress' | tr -d '"' | sed -e 's/null//g')
    CERT_EXTRA_SANS="--cert-extra-sans ${MASTERKUBE}.${DOMAIN_NAME},masterkube-aws.${DOMAIN_NAME},masterkube-aws-dashboard.${DOMAIN_NAME}"
    IP_TYPE="private"
fi

# Record Masterkube in Route53 DNS
if [ ! -z $ROUTE53_ZONEID ]; then
    PRIVATE_ADDR=$(echo ${LAUNCHED_INSTANCE} | jq '.PrivateIpAddress' | tr -d '"' | sed -e 's/null//g')
cat > config/${MASTERKUBE}-dns.json <<EOF
{
	"Comment": "${MASTERKUBE} private DNS entry",
	"Changes": [
		{
			"Action": "UPSERT",
			"ResourceRecordSet": {
				"Name": "${FQHOSTNAME}",
				"Type": "A",
				"TTL": 300,
				"ResourceRecords": [
                    {
                        "Value": "$PRIVATE_ADDR"
                    }
				]
			}
		}
	]
}
EOF
    aws route53 change-resource-record-sets --profile ${AWS_PROFILE} --region ${AWS_REGION} --hosted-zone-id ${ROUTE53_ZONEID} \
        --change-batch file://config/${MASTERKUBE}-dns.json
fi

echo -n "Wait for ${MASTERKUBE} ssh ready on ${IP_TYPE} IP=${IPADDR}"

while :
do
    ssh ${SSH_OPTIONS} -o ConnectTimeout=1 "${SEED_USER}@${IPADDR}" sudo hostnamectl set-hostname "${FQHOSTNAME}" 2>/dev/null && break
    echo -n "."
    sleep 1
done

echo

echo "Prepare ${MASTERKUBE} instance"
scp ${SSH_OPTIONS} -r ../masterkube ${SEED_USER}@${IPADDR}:~

echo "Start kubernetes ${MASTERKUBE} instance master node, kubernetes version=${KUBERNETES_VERSION}, providerID=${PROVIDERID}"
ssh ${SSH_OPTIONS} ${SEED_USER}@${IPADDR} sudo cp /home/${SEED_USER}/masterkube/bin/* /usr/local/bin
ssh ${SSH_OPTIONS} ${SEED_USER}@${IPADDR} sudo create-cluster.sh \
    --max-pods=${MAX_PODS} \
    --private-zone-id="${ROUTE53_ZONEID}" \
    --private-zone-name="${PRIVATE_DOMAIN_NAME}" \
    --cloud-provider=${CLOUD_PROVIDER} \
    --cni-plugin="${CNI_PLUGIN}" \
    --kubernetes-version="${KUBERNETES_VERSION}" \
    --node-group="${NODEGROUP_NAME}" ${CERT_EXTRA_SANS}

scp ${SSH_OPTIONS} ${SEED_USER}@${IPADDR}:/etc/cluster/* ./cluster

# Update /etc/hosts
if [ "${OSDISTRO}" == "Linux" ]; then
    sudo sed -i "/${MASTERKUBE}.${DOMAIN_NAME}/d" /etc/hosts
    sed -i -E "s/https:\/\/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:([0-9]+)/https:\/\/${MASTERKUBE}.${DOMAIN_NAME}:\1/g" cluster/config
else
    sudo sed -i'' "/${MASTERKUBE}.${DOMAIN_NAME}/d" /etc/hosts
    sed -i'' -E "s/https:\/\/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:([0-9]+)/https:\/\/${MASTERKUBE}.${DOMAIN_NAME}:\1/g" cluster/config
fi

sudo bash -c "echo '${IPADDR} ${MASTERKUBE}.${DOMAIN_NAME} masterkube-aws.${DOMAIN_NAME} masterkube-aws-dashboard.${DOMAIN_NAME}' >> /etc/hosts"

# Register in godaddy
gdip=$(curl -s -X GET -H "Authorization: sso-key ${GODADDY_API_KEY}:${GODADDY_API_SECRET}" "https://api.godaddy.com/v1/domains/${DOMAIN_NAME}/records/A/${MASTERKUBE}" | jq '.[0]|.data' | tr -d '"')

if [ "$gdip" != "$IPADDR" -a "$IPADDR" != "" ]; then
    curl -s -X PUT "https://api.godaddy.com/v1/domains/${DOMAIN_NAME}/records/A/${MASTERKUBE}" -H "Authorization: sso-key ${GODADDY_API_KEY}:${GODADDY_API_SECRET}" -H "Content-Type: application/json" -d "[{\"data\": \"${IPADDR}\"}]"
fi

MASTER_IP=$(cat ./cluster/manager-ip)
TOKEN=$(cat ./cluster/token)
CACERT=$(cat ./cluster/ca.cert)

kubectl create secret tls kube-system -n kube-system --key ./etc/ssl/privkey.pem --cert ./etc/ssl/fullchain.pem --kubeconfig=./cluster/config
kubectl create secret generic autoscaler-ssh-keys -n kube-system --from-file=id_rsa="${SSH_PRIVATE_KEY}" --from-file=id_rsa.pub="${SSH_PUBLIC_KEY}" --kubeconfig=./cluster/config

kubeconfig-merge.sh ${MASTERKUBE} ./cluster/config

echo "Write aws autoscaler provider config"

echo $(eval "cat <<EOF
$(<./templates/cluster/grpc-config.json)
EOF") | jq . >./config/grpc-config.json

AUTOSCALER_CONFIG=$(cat <<EOF
{
    "network": "${TRANSPORT}",
    "listen": "${LISTEN}",
    "secret": "${SCHEME}",
    "minNode": ${MINNODES},
    "maxNode": ${MAXNODES},
    "maxPods": ${MAX_PODS},
    "nodePrice": 0.0,
    "podPrice": 0.0,
    "image": "${TARGET_IMAGE}",
    "cloud-provider": "${CLOUD_PROVIDER}",
    "optionals": {
        "pricing": true,
        "getAvailableMachineTypes": true,
        "newNodeGroup": false,
        "templateNodeInfo": false,
        "createNodeGroup": false,
        "deleteNodeGroup": false
    },
    "kubeadm": {
        "address": "${MASTER_IP}",
        "token": "${TOKEN}",
        "ca": "sha256:${CACERT}",
        "extras-args": [
            "--ignore-preflight-errors=All"
        ]
    },
    "default-machine": "${DEFAULT_MACHINE}",
    "machines": ${MACHINES_TYPES},
    "sync-folder": {
    },
    "ssh-infos" : {
        "user": "${SEED_USER}",
        "ssh-private-key": "${SSH_PRIVATE_KEY_LOCAL}"
    },
    "aws": {
        "${NODEGROUP_NAME}": {
            "accessKey": "${AWS_ACCESSKEY}",
            "secretKey": "${AWS_SECRETKEY}",
            "token": "${AWS_TOKEN}",
            "profile": "${AWS_PROFILE}",
            "region" : "${AWS_REGION}",
            "keyName": "${SSH_KEYNAME}",
            "ami": "${TARGET_IMAGE_AMI}",
            "iam-role-arn": "${WORKER_INSTANCE_PROFILE_ARN}",
            "timeout": 120,
            "tags": [
                {
                    "key": "CustomTag",
                    "value": "CustomValue"
                }
            ],
            "network": {
                "route53": "$ROUTE53_ZONEID",
                "privateZoneName": "$PRIVATE_DOMAIN_NAME",
                "eni": [
                    {
                        "subnet": "${VPC_WORKER_SUBNET_ID}",
                        "securityGroup": "${VPC_WORKER_SECURITY_GROUPID}",
                        "publicIP": ${VPC_WORKER_USE_PUBLICIP}
                    }
                ]
            }
        }
    }
}
EOF
)

echo "${AUTOSCALER_CONFIG}" | jq . > config/kubernetes-aws-autoscaler.json

# Recopy config file on master node
kubectl create configmap config-cluster-autoscaler --kubeconfig=./cluster/config -n kube-system \
	--from-file ./config/grpc-config.json \
	--from-file ./config/kubernetes-aws-autoscaler.json

# Create Pods
create-ingress-controller.sh
create-dashboard.sh
create-metrics.sh
create-helloworld.sh
create-external-dns.sh

if [ "${LAUNCH_CA}" != "NO" ]; then
    create-autoscaler.sh ${LAUNCH_CA}
fi

# Add cluster config in configmap
kubectl create configmap masterkube-config --kubeconfig=./cluster/config -n kube-system \
	--from-file ./cluster/ca.cert \
    --from-file ./cluster/dashboard-token \
    --from-file ./cluster/token

popd
