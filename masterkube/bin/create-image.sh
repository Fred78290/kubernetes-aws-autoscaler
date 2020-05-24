#/bin/bash

set -e

KUBERNETES_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt)
CNI_VERSION=v0.8.5
SSH_KEY=$(cat ~/.ssh/id_rsa.pub)
CACHE=~/.local/aws/cache
TARGET_IMAGE=bionic-kubernetes-$KUBERNETES_VERSION
OSDISTRO=$(uname -s)
SSH_KEYNAME="aws-k8s-key"
CURDIR=$(dirname $0)
FORCE=NO
SEED_USER=ubuntu
SEED_IMAGE="<to be filled>"
VPC_ID="<to be filled>"
VPC_SUBNET_ID="<to be filled>"
VPC_SECURITY_GROUPID="<to be filled>"

VPC_USE_PUBLICIP=true

SSH_OPTIONS="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

source ${CURDIR}/aws.defs

if [ "$OSDISTRO" == "Linux" ]; then
    TZ=$(cat /etc/timezone)
else
    TZ=$(sudo systemsetup -gettimezone | awk '{print $2}')
fi

mkdir -p $CACHE

TEMP=`getopt -o fc:i:k:n:op:s:u:v: --long force,profile:,region:,vpc-id:,subnet-id:,sg-id:,use-public-ip:,user:,ami:,custom-image:,ssh-key:,ssh-key-name:,cni-plugin:,cni-version:,kubernetes-version: -n "$0" -- "$@"`
eval set -- "$TEMP"

# extract options and their arguments into variables.
while true ; do
	#echo "1:$1"
    case "$1" in
        -f|--force) FORCE=YES ; shift;;

        -p|--profile) AWS_PROFILE="${2}" ; shift 2;;
        -r|--region) AWS_REGION="${2}" ; shift 2;;
        -i|--custom-image) TARGET_IMAGE="$2" ; shift 2;;
        -k|--ssh-key) SSH_KEY=$2 ; shift 2;;
        -n|--cni-version) CNI_VERSION=$2 ; shift 2;;
        -u|--user) SEED_USER=$2 ; shift 2;;
        -v|--kubernetes-version) KUBERNETES_VERSION=$2 ; shift 2;;

        --ami) SEED_IMAGE=$2 ; shift 2;;
        --ssh-key-name) SSH_KEY_NAME=$2 ; shift 2;;
        --vpc-id) VC_NETWORK_PUBLIC="${2}" ; shift 2;;
        --subnet-id) VPC_SUBNET_ID="${2}" ; shift 2;;
        --sg-id) VPC_SECURITY_GROUPID="${2}" ; shift 2;;
        --use-public-ip) VPC_USE_PUBLICIP="${2}" ; shift 2;;

        --) shift ; break ;;
        *) echo "$1 - Internal error!" ; exit 1 ;;
    esac
done

TARGET_IMAGE_ID=$(aws ec2 describe-images --profile ${AWS_PROFILE} --region ${AWS_REGION} --filters "Name=architecture,Values=x86_64" "Name=name,Values=${TARGET_IMAGE}" "Name=virtualization-type,Values=hvm" 2>/dev/null | jq '.Images[0].ImageId' | tr -d '"' | sed -e 's/null//g')
SOURCE_IMAGE_ID=$(aws ec2 describe-images --profile ${AWS_PROFILE} --region ${AWS_REGION} --image-ids "${SEED_IMAGE}" 2>/dev/null | jq '.Images[0].ImageId' | tr -d '"' | sed -e 's/null//g')
KEYEXISTS=$(aws ec2 describe-key-pairs --profile ${AWS_PROFILE} --region ${AWS_REGION} --key-names "${SSH_KEYNAME}" 2>/dev/null | jq  '.KeyPairs[].KeyName' | tr -d '"')

if [ ! -z "${TARGET_IMAGE_ID}" ]; then
    if [ $FORCE = NO ]; then
        echo "$TARGET_IMAGE already exists!"
        exit 0
    fi
    aws ec2 deregister-image --profile ${AWS_PROFILE} --region ${AWS_REGION} --image-id "${TARGET_IMAGE_ID}" &>/dev/null
fi

if [ -z "${SOURCE_IMAGE_ID}" ]; then
    echo "Source $SOURCE_IMAGE_ID not found!"
    exit -1
fi

if [ -z ${KEYEXISTS} ]; then
    echo "SSH Public key doesn't exist"
    if [ -z ${SSH_KEY_PUB} ]; then
        echo "${SSH_KEY_PUB} doesn't exists. FATAL"
        exit -1
    fi
    aws ec2 import-key-pair --profile ${AWS_PROFILE} --region ${AWS_REGION} --key-name ${SSH_KEYNAME} --public-key-material "file://${SSH_KEY_PUB}"
fi

KUBERNETES_MINOR_RELEASE=$(echo -n $KUBERNETES_VERSION | tr '.' ' ' | awk '{ print $2 }')

echo "Prepare ${TARGET_IMAGE} image"

cat > $CACHE/mapping.json <<EOF
[
    {
        "DeviceName": "/dev/sda1",
        "Ebs": {
            "DeleteOnTermination": true,
            "VolumeType": "standard",
            "VolumeSize": 10,
            "Encrypted": false
        }
    }
]
EOF

cat > "${CACHE}/prepare-image.sh" <<EOF
#!/bin/bash
CNI_VERSION=${CNI_VERSION}
CNI_PLUGIN=${CNI_PLUGIN}
KUBERNETES_VERSION=${KUBERNETES_VERSION}
KUBERNETES_MINOR_RELEASE=${KUBERNETES_MINOR_RELEASE}
ECR_PASSWORD=${ECR_PASSWORD}
EOF

cat >> "${CACHE}/prepare-image.sh" <<"EOF"

function pull_image() {
    DOCKER_IMAGES=$(curl -s $1 | grep "image: " | sed -E 's/.+image: (.+)/\1/g')

    for DOCKER_IMAGE in $DOCKER_IMAGES
    do
        echo "Pull image $DOCKER_IMAGE"
        docker pull $DOCKER_IMAGE
    done
}

apt-get update
apt-get upgrade -y
apt-get autoremove -y
apt-get install jq socat conntrack -y

mkdir -p /opt/cni/bin
mkdir -p /usr/local/bin

echo "Prepare to install Docker"

# Setup daemon.
if [ $KUBERNETES_MINOR_RELEASE -ge 14 ]; then
    mkdir -p /etc/docker

    cat > /etc/docker/daemon.json <<SHELL
{
    "exec-opts": [
        "native.cgroupdriver=systemd"
    ],
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "100m"
    },
    "storage-driver": "overlay2"
}
SHELL

    curl https://get.docker.com | bash

    mkdir -p /etc/systemd/system/docker.service.d

    # Restart docker.
    systemctl daemon-reload
    systemctl restart docker
else
    curl https://get.docker.com | bash
fi

echo "Prepare to install CNI plugins"

curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-amd64-${CNI_VERSION}.tgz" | tar -C /opt/cni/bin -xz

cd /usr/local/bin
curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/bin/linux/amd64/{kubeadm,kubelet,kubectl,kube-proxy}
chmod +x /usr/local/bin/kube*

mkdir -p /etc/systemd/system/kubelet.service.d

cat > /etc/systemd/system/kubelet.service <<SHELL
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/

[Service]
ExecStart=/usr/local/bin/kubelet
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
SHELL

cat > /etc/systemd/system/kubelet.service.d/10-kubeadm.conf <<"SHELL"
# Note: This dropin only works with kubeadm and kubelet v1.11+
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
# This is a file that "kubeadm init" and "kubeadm join" generate at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably, the user should use
# the .NodeRegistration.KubeletExtraArgs object in the configuration files instead. KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/default/kubelet
ExecStart=
ExecStart=/usr/local/bin/kubelet $KUBELET_KUBECONFIG_ARGS $KUBELET_CONFIG_ARGS $KUBELET_KUBEADM_ARGS $KUBELET_EXTRA_ARGS
SHELL

#echo 'KUBELET_EXTRA_ARGS="--network-plugin=cni --fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true"' > /etc/default/kubelet
echo 'KUBELET_EXTRA_ARGS=' > /etc/default/kubelet

echo 'export PATH=/opt/cni/bin:$PATH' >> /etc/profile.d/apps-bin-path.sh

systemctl enable kubelet
systemctl restart kubelet

usermod -aG docker ubuntu

if [ "$CNI_PLUGIN" != "aws" ]; then
    modprobe br_netfilter
    echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf
fi

/usr/local/bin/kubeadm config images pull --kubernetes-version=${KUBERNETES_VERSION}

if [ "$CNI_PLUGIN" = "aws" ]; then
    docker login -u AWS -p "$ECR_PASSWORD" "602401143452.dkr.ecr.us-west-2.amazonaws.com"
    pull_image https://raw.githubusercontent.com/aws/amazon-vpc-cni-k8s/release-1.6/config/v1.6/aws-k8s-cni.yaml
elif [ "$CNI_PLUGIN" = "calico" ]; then
    pull_image https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/etcd.yaml
    pull_image https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/calico.yaml 2>&1
    pull_image https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/kubernetes-datastore/calicoctl.yaml
elif [ "$CNI_PLUGIN" = "flannel" ]; then
    pull_image https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml 2>&1
elif [ "$CNI_PLUGIN" = "weave" ]; then
    pull_image "https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')"
elif [ "$CNI_PLUGIN" = "canal" ]; then
    pull_image https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/rbac.yaml 2>&1
    pull_image https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/canal.yaml 2>&1
elif [ "$CNI_PLUGIN" = "kube" ]; then
    pull_image https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter.yaml
    pull_image https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter-all-features.yaml
elif [ "$CNI_PLUGIN" = "romana" ]; then
    pull_image https://raw.githubusercontent.com/romana/romana/master/containerize/specs/romana-kubeadm.yml
fi

[ -f /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg ] && rm /etc/cloud/cloud.cfg.d/50-curtin-networking.cfg
rm /etc/netplan/*
cloud-init clean
rm /var/log/cloud-ini*
rm /var/log/syslog
EOF

chmod +x "${CACHE}/prepare-image.sh"

if [ "${VPC_USE_PUBLICIP}" == "true" ]; then
    PUBLIC_IP_OPTIONS=--associate-public-ip-address
else
    PUBLIC_IP_OPTIONS=--no-associate-public-ip-address
fi

echo "Launch instance ${SEED_IMAGE} to ${TARGET_IMAGE}"
LAUNCHED_INSTANCE=$(aws ec2 run-instances \
    --profile ${AWS_PROFILE} \
    --region ${AWS_REGION} \
    --image-id ${SEED_IMAGE} \
    --count 1  \
    --instance-type t2.micro \
    --key-name ${SSH_KEYNAME} \
    --subnet-id ${VPC_SUBNET_ID} \
    --security-group-ids ${VPC_SECURITY_GROUPID} \
    --block-device-mappings "file://${CACHE}/mapping.json" \
    --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=${TARGET_IMAGE}}]" \
    ${PUBLIC_IP_OPTIONS})

#echo $LAUNCHED_INSTANCE | jq .

LAUNCHED_ID=$(echo ${LAUNCHED_INSTANCE} | jq '.Instances[0].InstanceId' | tr -d '"' | sed -e 's/null//g')

if [ -z ${LAUNCHED_ID} ]; then
    echo "Something goes wrong when launching ${TARGET_IMAGE}"
    exit -1
fi

echo -n "Wait for ${TARGET_IMAGE} instanceID ${LAUNCHED_ID} to boot"

while [ ! "$(aws ec2  describe-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids ${LAUNCHED_ID} | jq .Reservations[0].Instances[0].State.Code)" -eq 16 ];
do
    echo -n "."
    sleep 1
done

echo

LAUNCHED_INSTANCE=$(aws ec2  describe-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids ${LAUNCHED_ID} | jq .Reservations[0].Instances[0])

if [ "${VPC_USE_PUBLICIP}" == "true" ]; then
    export IPADDR=$(echo ${LAUNCHED_INSTANCE} | jq '.PublicIpAddress' | tr -d '"' | sed -e 's/null//g')
    IP_TYPE="public"
else
    export IPADDR=$(echo ${LAUNCHED_INSTANCE} | jq '.PrivateIpAddress' | tr -d '"' | sed -e 's/null//g')
    IP_TYPE="private"
fi

echo -n "Wait for ${TARGET_IMAGE} ssh ready for on ${IP_TYPE} IP=${IPADDR}"

while :
do
    echo -n "."
    scp ${SSH_OPTIONS} -o ConnectTimeout=1 "${CACHE}/prepare-image.sh" "${SEED_USER}@${IPADDR}":~ 2>/dev/null && break
    sleep 1
done

echo

ssh ${SSH_OPTIONS} -t "${SEED_USER}@${IPADDR}" sudo ./prepare-image.sh
ssh ${SSH_OPTIONS} -t "${SEED_USER}@${IPADDR}" rm ./prepare-image.sh

aws ec2 stop-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids "${LAUNCHED_ID}" &> /dev/null

echo -n "Wait ${TARGET_IMAGE} to shutdown"
while [ ! $(aws ec2  describe-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids "${LAUNCHED_ID}" | jq .Reservations[0].Instances[0].State.Code) -eq 80 ];
do
    echo -n "."
    sleep 1
done
echo

echo "Created image ${TARGET_IMAGE} with kubernetes version ${KUBERNETES_VERSION}"

IMAGEID=$(aws ec2 create-image --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-id "${LAUNCHED_ID}" --name "${TARGET_IMAGE}" --description "Kubernetes ${KUBERNETES_VERSION} image ready to use, based on AMI ${SEED_IMAGE}" | jq .ImageId | tr -d '"' | sed -e 's/null//g')

if [ -z $IMAGEID ]; then
    echo "Something goes wrong when creating image from ${TARGET_IMAGE}"
    exit -1
fi

echo -n "Wait AMI ${IMAGEID} to be available"
while [ ! $(aws ec2 describe-images --profile ${AWS_PROFILE} --region ${AWS_REGION} --image-ids "${IMAGEID}" | jq .Images[0].State | tr -d '"') == "available" ];
do
    echo -n "."
    sleep 5
done
echo

aws ec2 terminate-instances --profile ${AWS_PROFILE} --region ${AWS_REGION} --instance-ids "${LAUNCHED_ID}" &>/dev/null

exit 0
