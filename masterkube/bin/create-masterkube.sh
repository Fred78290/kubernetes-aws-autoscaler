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
export SSH_KEY=$(cat "${SSH_PRIVATE_KEY}.pub")
export KUBERNETES_VERSION=v1.18.2
export KUBECONFIG=${HOME}/.kube/config
export ROOT_IMG_NAME=bionic-kubernetes
export TARGET_IMAGE="${ROOT_IMG_NAME}-${KUBERNETES_VERSION}"
export CNI_VERSION="v0.8.5"
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

export SEED_USER="<to be filled>"
export SEED_IMAGE="<to be filled>"
export IAM_ROLE_ARN="<to be filled>"
export VPC_ID="<to be filled>"
export VPC_SUBNET_ID="<to be filled>"
export VPC_SECURITY_GROUPID="<to be filled>"
export VPC_USE_PUBLICIP=true

export LAUNCH_CA=YES

source ${CURDIR}/aws.defs

# Use public IP address only if we run autoscaler outside AWS
if [ "${LAUNCH_CA}" != "YES" ]; then
    export AUTOSCALER_USE_PUBLICIP=true
else
    export AUTOSCALER_USE_PUBLICIP=false
fi

SSH_OPTIONS="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

if [ "${OSDISTRO}" == "Linux" ]; then
    TZ=$(cat /etc/timezone)
    BASE64="base64 -w 0"
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
    BASE64="base64"
fi

TEMP=$(getopt -o p:r:k:n:p:s:t: --long profile:,region:,node-group:,target-image:,seed-image:,seed-user:,vpc-id:,subnet-id:,sg-id:,transport:,ssh-private-key:,cni-version:,kubernetes-version:,max-nodes-total:,cores-total:,memory-total:,max-autoprovisioned-node-group-count:,scale-down-enabled:,scale-down-delay-after-add:,scale-down-delay-after-delete:,scale-down-delay-after-failure:,scale-down-unneeded-time:,scale-down-unready-time:,unremovable-node-recheck-timeout: -n "$0" -- "$@")

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

    --node-group)
        NODEGROUP_NAME="$2"
        MASTERKUBE="${NODEGROUP_NAME}-masterkube"
        PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
        shift 2
        ;;

    --target-image)
        ROOT_IMG_NAME="$2"
        TARGET_IMAGE="${ROOT_IMG_NAME}-${KUBERNETES_VERSION}"
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

    --vpc-id)
        VPC_ID="$2"
        shift 2
        ;;

    --subnet-id)
        VPC_SUBNET_ID="$2"
        shift 2
        ;;

    --sg-id)
        VPC_SECURITY_GROUPID="$2"
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
    -t | --transport)
        TRANSPORT="$2"
        shift 2
        ;;
    -k | --kubernetes-version)
        KUBERNETES_VERSION="$2"
        TARGET_IMAGE="${ROOT_IMG_NAME}-${KUBERNETES_VERSION}"
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

KEYEXISTS=$(aws ec2 describe-key-pairs --profile ${AWS_PROFILE} --region ${AWS_REGION} --key-names "${SSH_KEYNAME}" | jq  '.KeyPairs[].KeyName' | tr -d '"')

if [ -z ${KEYEXISTS} ]; then
    echo "SSH Public key doesn't exist"
    if [ ! -f ${SSH_KEY_PUB} ]; then
        echo "${SSH_KEY_PUB} doesn't exists. FATAL"

        exit -1
    fi
    aws ec2 import-key-pair --profile ${AWS_PROFILE} --region ${AWS_REGION} --key-name ${SSH_KEYNAME} --public-key-material "file://${SSH_KEY_PUB}"
else
    echo "SSH Public key already exists"
fi

export SSH_KEY_FNAME=$(basename ${SSH_PRIVATE_KEY})
export SSH_KEY_PUB="${SSH_PRIVATE_KEY}.pub"
export SSH_KEY=$(cat "${SSH_KEY_PUB}")

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
    SSH_PRIVATE_KEY_LOCAL="/etc/cluster/${SSH_KEY_FNAME}"
    TRANSPORT=unix
    LISTEN="/var/run/cluster-autoscaler/aws.sock"
    CONNECTTO="unix:/var/run/cluster-autoscaler/aws.sock"
fi

echo "Transport set to:${TRANSPORT}, listen endpoint at ${LISTEN}"

pushd ${CURDIR}/../

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

# If the VM template doesn't exists, build it from scrash
if [ -z "${TARGET_IMAGE_AMI}" ]; then
    echo "Create aws preconfigured image ${TARGET_IMAGE}"

    ./bin/create-image.sh \
        --profile="${AWS_PROFILE}" \
        --region="${AWS_REGION}" \
        --cni-version="${CNI_VERSION}" \
        --custom-image="${TARGET_IMAGE}" \
        --kubernetes-version="${KUBERNETES_VERSION}" \
        --ami="${SEED_IMAGE}" \
        --user="${SEED_USER}" \
        --ssh-key="${SSH_KEY}" \
        --ssh-key-name="${SSH_KEYNAME}" \
        --vpc-id="${VC_NETWORK_PUBLIC}" \
        --subnet-id="${VPC_SUBNET_ID}" \
        --sg-id="${VPC_SECURITY_GROUPID}" \
        --use-public-ip="${VPC_USE_PUBLICIP}"
fi

export TARGET_IMAGE_AMI=$(aws ec2 describe-images --profile ${AWS_PROFILE} --region ${AWS_REGION} --filters "Name=name,Values=${TARGET_IMAGE}" | jq '.Images[0].ImageId' | tr -d '"' | sed -e 's/null//g')

if [ -d ${TARGET_IMAGE_AMI} ]; then
    echo "AMI ${TARGET_IMAGE} not found"
    exit -1
fi

# Delete previous exixting version
delete-masterkube.sh

echo "Launch custom ${MASTERKUBE} instance with ${TARGET_IMAGE}"

# Cloud init user-data
echo "#cloud-config" >./config/userdata.yaml
cat <<EOF | python2 -c "import json,sys,yaml; print yaml.safe_dump(json.load(sys.stdin), width=500, indent=4, default_flow_style=False)" >>./config/userdata.yaml
{
    "runcmd": [
        "echo 'Create ${MASTERKUBE}' > /var/log/masterkube.log",
        "hostnamectl set-hostname ${MASTERKUBE}"
    ]
}
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

if [ "${VPC_USE_PUBLICIP}" == "true" ]; then
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
    --subnet-id "${VPC_SUBNET_ID}" \
    --security-group-ids "${VPC_SECURITY_GROUPID}" \
    --user-data "file://config/userdata.yaml" \
    --iam-instance-profile "Arn=${IAM_ROLE_ARN}" \
    --block-device-mappings "file://config/mapping.json" \
    --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=${MASTERKUBE}},{Key=NodeGroup,Value=${NODEGROUP_NAME}},{Key=kubernetes.io/cluster/${NODEGROUP_NAME},Value=owned},{Key=KubernetesCluster,Value=${NODEGROUP_NAME}]" \
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

if [ "${VPC_USE_PUBLICIP}" == "true" ]; then
    export IPADDR=$(echo ${LAUNCHED_INSTANCE} | jq '.PublicIpAddress' | tr -d '"' | sed -e 's/null//g')
    CERT_EXTRA_SANS="--cert-extra-sans ${IPADDR},${MASTERKUBE}.${DOMAIN_NAME},masterkube.${DOMAIN_NAME},masterkube-dashboard.${DOMAIN_NAME}"
    IP_TYPE="public"
else
    export IPADDR=$(echo ${LAUNCHED_INSTANCE} | jq '.PrivateIpAddress' | tr -d '"' | sed -e 's/null//g')
    CERT_EXTRA_SANS="--cert-extra-sans ${MASTERKUBE}.${DOMAIN_NAME},masterkube.${DOMAIN_NAME},masterkube-dashboard.${DOMAIN_NAME}"
    IP_TYPE="private"
fi

echo -n "Wait for ${MASTERKUBE} ssh ready on ${IP_TYPE} IP=${IPADDR}"

while :
do
    ssh ${SSH_OPTIONS} -o ConnectTimeout=1 "${SEED_USER}@${IPADDR}" sudo hostnamectl set-hostname "${MASTERKUBE}" 2>/dev/null && break
    echo -n "."
    sleep 1
done

echo

echo "Prepare ${MASTERKUBE} instance"
scp ${SSH_OPTIONS} -r ../masterkube ${SEED_USER}@${IPADDR}:~

echo "Start kubernetes ${MASTERKUBE} instance master node, kubernetes version=${KUBERNETES_VERSION}, providerID=${PROVIDERID}"
ssh ${SSH_OPTIONS} ${SEED_USER}@${IPADDR} sudo cp /home/${SEED_USER}/masterkube/bin/* /usr/local/bin
ssh ${SSH_OPTIONS} ${SEED_USER}@${IPADDR} sudo create-cluster.sh --cni-plugin $CNI_PLUGIN --kubernetes-version "${KUBERNETES_VERSION}" --node-group "${NODEGROUP_NAME}" ${CERT_EXTRA_SANS}

scp ${SSH_OPTIONS} ${SEED_USER}@${IPADDR}:/etc/cluster/* ./cluster

# Update /etc/hosts
if [ "${OSDISTRO}" == "Linux" ]; then
    sudo sed -i "/masterkube.${DOMAIN_NAME}/d" /etc/hosts
    sed -i -E "s/https:\/\/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:([0-9]+)/https:\/\/${MASTERKUBE}.${DOMAIN_NAME}:\1/g" cluster/config
else
    sudo sed -i'' "/masterkube.${DOMAIN_NAME}/d" /etc/hosts
    sed -i'' -E "s/https:\/\/[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+:([0-9]+)/https:\/\/${MASTERKUBE}.${DOMAIN_NAME}:\1/g" cluster/config
fi

sudo bash -c "echo '${IPADDR} ${MASTERKUBE}.${DOMAIN_NAME} masterkube.${DOMAIN_NAME} masterkube-dashboard.${DOMAIN_NAME}' >> /etc/hosts"

MASTER_IP=$(cat ./cluster/manager-ip)
TOKEN=$(cat ./cluster/token)
CACERT=$(cat ./cluster/ca.cert)

kubectl annotate node ${MASTERKUBE} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "cluster.autoscaler.nodegroup/instance-id=${LAUNCHED_ID}" "cluster.autoscaler.nodegroup/node-index=0" "cluster.autoscaler.nodegroup/autoprovision=false" "cluster-autoscaler.kubernetes.io/scale-down-disabled=true" --overwrite --kubeconfig=./cluster/config
kubectl label nodes ${MASTERKUBE} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "master=true" --overwrite --kubeconfig=./cluster/config
kubectl create secret tls kube-system -n kube-system --key ./etc/ssl/privkey.pem --cert ./etc/ssl/fullchain.pem --kubeconfig=./cluster/config

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
    "nodePrice": 0.0,
    "podPrice": 0.0,
    "image": "${TARGET_IMAGE}",
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
    "machines": {
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
    },
    "cloud-init": {
        "package_update": false,
        "package_upgrade": false
    },
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
            "iam-role-arn": "${IAM_ROLE_ARN}",
            "timeout": 120,
            "tags": [
                {
                    "key": "CustomTag",
                    "value": "CustomValue"
                }
            ],
            "network": {
                "autoScalerUsePublicIP": ${AUTOSCALER_USE_PUBLICIP},
                "eni": [
                    {
                        "subnet": "${VPC_SUBNET_ID}",
                        "securityGroup": "${VPC_SECURITY_GROUPID}",
                        "publicIP": ${VPC_USE_PUBLICIP}
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
scp ${SSH_OPTIONS} ${SSH_PRIVATE_KEY} ./config/grpc-config.json ./config/kubernetes-aws-autoscaler.json ${SEED_USER}@${IPADDR}:/tmp
ssh ${SSH_OPTIONS} ${SEED_USER}@${IPADDR} sudo cp "/tmp/${SSH_KEY_FNAME}" /tmp/grpc-config.json /tmp/kubernetes-aws-autoscaler.json /etc/cluster

# Create Pods
create-ingress-controller.sh
create-dashboard.sh
create-helloworld.sh

if [ "${LAUNCH_CA}" != "NO" ]; then
    create-autoscaler.sh ${LAUNCH_CA}
fi

popd
