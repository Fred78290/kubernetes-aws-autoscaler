#!/bin/bash

set -e

export CNI=aws
export KUBERNETES_VERSION=v1.18.2
export CLUSTER_DIR=/etc/cluster
export SCHEME="aws"
export NODEGROUP_NAME="aws-ca-k8s"
export MASTERKUBE="${NODEGROUP_NAME}-masterkube"
export PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${HOSTNAME}"
export IPADDR=$(curl http://169.254.169.254/latest/meta-data/local-ipv4)
export LOCALHOSTNAME=$(curl -s http://169.254.169.254/latest/meta-data/local-hostname)
export AWS_DOMAIN=${LOCALHOSTNAME#*.*}
export MAC_ADDRESS="$(curl -s http://169.254.169.254/latest/meta-data/mac)"
export SUBNET_IPV4_CIDR_BLOCK=$(curl -s http://169.254.169.254/latest/meta-data/network/interfaces/macs/${MAC_ADDRESS}/subnet-ipv4-cidr-block)
export VPC_IPV4_CIDR_BLOCK=$(curl -s http://169.254.169.254/latest/meta-data/network/interfaces/macs/${MAC_ADDRESS}/vpc-ipv4-cidr-block)
export DNS_SERVER=$(echo $VPC_IPV4_CIDR_BLOCK | tr './' ' '| awk '{print $1"."$2"."$3".2"}')
export KUBECONFIG=/etc/kubernetes/admin.conf
export KUBEADM_CONFIG=/etc/kubernetes/kubeadm-config.yaml
export K8_OPTIONS="--ignore-preflight-errors=All --config ${KUBEADM_CONFIG}"
export KUBEADM_TOKEN=$(kubeadm token generate)
export APISERVER_ADVERTISE_ADDRESS="${IPADDR}"
export APISERVER_ADVERTISE_PORT="6443"
export TOKEN_TLL="0s"
export POD_NETWORK_CIDR="10.244.0.0/16"
export SERVICE_NETWORK_CIDR="10.96.0.0/12"
export CLUSTER_DNS="10.96.0.10"
export CERT_EXTRA_SANS=

TEMP=$(getopt -o n:c:k:s:i: --long node-group:,cert-extra-sans:,cni-plugin:,cni-version:,kubernetes-version: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -n | --node-group)
        NODEGROUP_NAME="$2"
        MASTERKUBE="${NODEGROUP_NAME}-masterkube"
        PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${HOSTNAME}"
        shift 2
        ;;

    -i | --cni-version)
        CNI_VERSION="$2"
        shift 2
        ;;

    -c | --cni-plugin)
        CNI="$2"
        shift 2
        ;;

    -k | --kubernetes-version)
        KUBERNETES_VERSION="$2"
        shift 2
        ;;

    -s | --cert-extra-sans)
        CERT_EXTRA_SANS="echo $2 | tr ',' ' '"
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

mkdir -p $CLUSTER_DIR
mkdir -p /etc/kubernetes

echo -n "$IPADDR:6443" > $CLUSTER_DIR/manager-ip

sed -i "2i${IPADDR} $(hostname)" /etc/hosts

if [ -f /etc/kubernetes/kubelet.conf ]; then
    echo "Already installed k8s master node"
fi

source /etc/default/kubelet

echo "KUBELET_EXTRA_ARGS='${KUBELET_EXTRA_ARGS} --node-ip=${IPADDR}'" > /etc/default/kubelet

systemctl restart kubelet

if [ -z "$CNI" ]; then
    CNI="flannel"
fi

CNI=$(echo "$CNI" | tr '[:upper:]' '[:lower:]')

case $CNI in
    aws)
        POD_NETWORK_CIDR="${SUBNET_IPV4_CIDR_BLOCK}"
        ;;
    flannel|weave|canal|kube)
        [ -e /proc/sys/net/bridge/bridge-nf-call-iptables ] && sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf
    ;;
    calico)
        echo "Download calicoctl"

        curl -s -O -L https://github.com/projectcalico/calicoctl/releases/download/v3.1.0/calicoctl
        chmod +x calicoctl
        mv calicoctl /usr/local/bin
        ;;
    *)
        echo "CNI $CNI is not supported"
        exit -1
        ;;
esac

cat > ${KUBEADM_CONFIG} <<EOF
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
bootstrapTokens:
- groups:
  - system:bootstrappers:kubeadm:default-node-token
  token: ${KUBEADM_TOKEN}
  ttl: ${TOKEN_TLL}
  usages:
  - signing
  - authentication
localAPIEndpoint:
  advertiseAddress: ${APISERVER_ADVERTISE_ADDRESS}
  bindPort: ${APISERVER_ADVERTISE_PORT}
nodeRegistration:
  criSocket: /var/run/dockershim.sock
  name: primary
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
  kubeletExtraArgs:
    network-plugin: cni
    provider-id: ${PROVIDERID}
    cloud-provider: aws
---
kind: KubeletConfiguration
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
clusterDNS:
- ${CLUSTER_DNS}
failSwapOn: false
featureGates:
  VolumeSubpathEnvExpansion: true
readOnlyPort: 10255
clusterDomain: cluster.local
cpuManagerReconcilePeriod: 0s
evictionPressureTransitionPeriod: 0s
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageMinimumGCAge: 0s
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
rotateCertificates: true
runtimeRequestTimeout: 0s
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
volumeStatsAggPeriod: 0s
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
certificatesDir: /etc/kubernetes/pki
clusterName: ${NODEGROUP_NAME}
dns:
  type: CoreDNS
etcd:
  local:
    dataDir: /var/lib/etcd
imageRepository: k8s.gcr.io
kubernetesVersion: ${KUBERNETES_VERSION}
controlPlaneEndpoint: ${APISERVER_ADVERTISE_ADDRESS}:${APISERVER_ADVERTISE_PORT}
networking:
  dnsDomain: cluster.local
  serviceSubnet: ${SERVICE_NETWORK_CIDR}
  podSubnet: ${POD_NETWORK_CIDR}
scheduler: {}
controllerManager:
  extraArgs:
    cloud-provider: aws
    configure-cloud-routes: "false"
apiServer:
  extraArgs:
    authorization-mode: Node,RBAC
    cloud-provider: aws
  timeoutForControlPlane: 4m0s
  certSANs:
  - ${IPADDR}
  - ${HOSTNAME}
  - ${LOCALHOSTNAME}
EOF

for CERT_EXTRA in $CERT_EXTRA_SANS
do
    echo "  - $CERT_EXTRA" >> ${KUBEADM_CONFIG}
done

echo "Init K8 cluster with options:$K8_OPTIONS, PROVIDERID=${PROVIDERID}"

cat ${KUBEADM_CONFIG}

kubeadm init $K8_OPTIONS 2>&1

echo "Retrieve token infos"

openssl x509 -pubkey -in /etc/kubernetes/pki/ca.crt | openssl rsa -pubin -outform der 2>/dev/null | openssl dgst -sha256 -hex | sed 's/^.* //' | tr -d '\n' > $CLUSTER_DIR/ca.cert
kubeadm token list 2>&1 | grep "authentication,signing" | awk '{print $1}'  | tr -d '\n' > $CLUSTER_DIR/token 

echo "Set local K8 environement"

mkdir -p $HOME/.kube
cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
chown $(id -u):$(id -g) $HOME/.kube/config

cp /etc/kubernetes/admin.conf $CLUSTER_DIR/config

chmod +r $CLUSTER_DIR/*

echo "Allow master to host pod"
kubectl taint nodes --all node-role.kubernetes.io/master- 2>&1

if [ "$CNI" = "aws" ]; then

    echo "Install AWS network"

    kubectl apply -f https://raw.githubusercontent.com/aws/amazon-vpc-cni-k8s/release-1.6/config/v1.6/aws-k8s-cni.yaml 2>&1

elif [ "$CNI" = "calico" ]; then

    echo "Install calico network"

    kubectl apply -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/etcd.yaml 2>&1
    
    kubectl apply -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/rbac.yaml 2>&1

    kubectl apply -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/calico.yaml 2>&1

    kubectl apply -f https://docs.projectcalico.org/v3.2/getting-started/kubernetes/installation/hosted/kubernetes-datastore/calicoctl.yaml 2>&1

elif [ "$CNI" = "flannel" ]; then

    echo "Install flannel network"

    kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml 2>&1

elif [ "$CNI" = "weave" ]; then

    echo "Install weave network for K8"

    kubectl apply -f "https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')" 2>&1

elif [ "$CNI" = "canal" ]; then

    echo "Install canal network"

    kubectl apply -f https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/rbac.yaml 2>&1
    kubectl apply -f https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/canal.yaml 2>&1

elif [ "$CNI" = "kube" ]; then

    echo "Install kube network"

    kubectl apply -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter.yaml 2>&1
    kubectl apply -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter-all-features.yaml 2>&1

elif [ "$CNI" = "romana" ]; then

    echo "Install romana network"

    kubectl apply -f https://raw.githubusercontent.com/romana/romana/master/containerize/specs/romana-kubeadm.yml 2>&1

fi

echo "Done k8s master node"
