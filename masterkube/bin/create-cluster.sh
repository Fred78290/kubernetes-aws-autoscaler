#!/bin/bash

set -e

export CNI_PLUGIN=aws
export CLOUD_PROVIDER=aws
export KUBERNETES_VERSION=v1.18.6
export CLUSTER_DIR=/etc/cluster
export SCHEME="aws"
export NODEGROUP_NAME="aws-ca-k8s"
export MASTERKUBE="${NODEGROUP_NAME}-masterkube"
export PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
export IPADDR=$(curl http://169.254.169.254/latest/meta-data/local-ipv4)
export LOCALHOSTNAME=$(curl -s http://169.254.169.254/latest/meta-data/local-hostname)
export INSTANCEID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
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
export MAX_PODS=110
export PRIVATE_DOMAIN_NAME=
export ROUTE53_ZONEID=

TEMP=$(getopt -o p:n:c:k:s: --long private-zone-id:,private-zone-name:,cloud-provider:,max-pods:,node-group:,cert-extra-sans:,cni-plugin:,kubernetes-version: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -p | --max-pods)
        MAX_PODS=$2
        shift 2
        ;;
    -n | --node-group)
        NODEGROUP_NAME="$2"
        MASTERKUBE="${NODEGROUP_NAME}-masterkube"
        PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
        shift 2
        ;;

    -c | --cni-plugin)
        CNI_PLUGIN="$2"
        shift 2
        ;;

    -k | --kubernetes-version)
        KUBERNETES_VERSION="$2"
        shift 2
        ;;

    -s | --cert-extra-sans)
        CERT_EXTRA_SANS="$2"
        shift 2
        ;;

    --cloud-provider)
        CLOUD_PROVIDER="$2"
        shift 2
        ;;

    --private-zone-id)
        ROUTE53_ZONEID="$2"
        shift 2
        ;;

    --private-zone-name)
        PRIVATE_DOMAIN_NAME="$2"
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

systemctl restart kubelet

if [ -z "$CNI_PLUGIN" ]; then
    CNI_PLUGIN="flannel"
fi

CNI_PLUGIN=$(echo "$CNI_PLUGIN" | tr '[:upper:]' '[:lower:]')

if [ $CLOUD_PROVIDER == "aws" ]; then
  KUBELET_EXTRA_ARGS="${KUBELET_EXTRA_ARGS} --cloud-provider=${CLOUD_PROVIDER} --node-ip=${IPADDR}"
  NODENAME=$LOCALHOSTNAME
else
  NODENAME=$MASTERKUBE
  KUBELET_EXTRA_ARGS="${KUBELET_EXTRA_ARGS} --node-ip=${IPADDR}"
fi

echo "KUBELET_EXTRA_ARGS='${KUBELET_EXTRA_ARGS}'" > /etc/default/kubelet

case $CNI_PLUGIN in
    aws)
        POD_NETWORK_CIDR="${SUBNET_IPV4_CIDR_BLOCK}"

        MAC=$(curl -s http://169.254.169.254/latest/meta-data/network/interfaces/macs/ -s | head -n 1 | sed 's/\/$//')
        TEN_RANGE=$(curl -s http://169.254.169.254/latest/meta-data/network/interfaces/macs/$MAC/vpc-ipv4-cidr-blocks | grep -c '^10\..*' || true )

        if [[ "$TEN_RANGE" != "0" ]]; then
          SERVICE_NETWORK_CIDR="172.20.0.0/16"
          CLUSTER_DNS="172.20.0.10"
        else
          CLUSTER_DNS="10.100.0.10"
          SERVICE_NETWORK_CIDR="10.100.0.0/16"
        fi
        ;;
    flannel|weave|canal|kube)
        POD_NETWORK_CIDR="10.244.0.0/16"
        ;;
    calico)
        echo "Download calicoctl"

        POD_NETWORK_CIDR="192.168.0.0/16"

        curl -s -O -L "https://github.com/projectcalico/calicoctl/releases/download/v3.14.1/calicoctl-linux-amd64"
        chmod +x calicoctl-linux-amd64
        mv calicoctl-linux-amd64 /usr/local/bin/calicoctl
        ;;
    *)
        echo "CNI_PLUGIN '$CNI_PLUGIN' is not supported"
        exit -1
        ;;
esac

if [ $CLOUD_PROVIDER = "aws" ]; then
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
  name: ${NODENAME}
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
hairpinMode: hairpin-veth
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
maxPods: ${MAX_PODS}
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
else
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
  name: ${NODENAME}
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
  kubeletExtraArgs:
    network-plugin: cni
    provider-id: ${PROVIDERID}
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
hairpinMode: hairpin-veth
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
maxPods: ${MAX_PODS}
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
apiServer:
  timeoutForControlPlane: 4m0s
  certSANs:
  - ${IPADDR}
  - ${HOSTNAME}
  - ${LOCALHOSTNAME}
EOF
fi

for CERT_EXTRA in $(tr ',' ' ' <<<$CERT_EXTRA_SANS) 
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

if [ "$CNI_PLUGIN" = "aws" ]; then

    echo "Install AWS network"

    kubectl apply -f "https://raw.githubusercontent.com/aws/amazon-vpc-cni-k8s/release-1.6.2/config/v1.6/aws-k8s-cni.yaml" 2>&1

elif [ "$CNI_PLUGIN" = "calico" ]; then

    echo "Install calico network"

    kubectl apply -f "https://docs.projectcalico.org/manifests/calico-vxlan.yaml" 2>&1

elif [ "$CNI_PLUGIN" = "flannel" ]; then

    echo "Install flannel network"

    kubectl apply -f "https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml" 2>&1

elif [ "$CNI_PLUGIN" = "weave" ]; then

    echo "Install weave network for K8"

    kubectl apply -f "https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')" 2>&1

elif [ "$CNI_PLUGIN" = "canal" ]; then

    echo "Install canal network"

    kubectl apply -f "https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/rbac.yaml" 2>&1
    kubectl apply -f "https://raw.githubusercontent.com/projectcalico/canal/master/k8s-install/1.7/canal.yaml" 2>&1

elif [ "$CNI_PLUGIN" = "kube" ]; then

    echo "Install kube network"

    kubectl apply -f "https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter.yaml" 2>&1
    kubectl apply -f "https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter-all-features.yaml" 2>&1

elif [ "$CNI_PLUGIN" = "romana" ]; then

    echo "Install romana network"

    kubectl apply -f https://raw.githubusercontent.com/romana/romana/master/containerize/specs/romana-kubeadm.yml 2>&1

fi

kubectl annotate node ${NODENAME} \
  "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" \
  "cluster.autoscaler.nodegroup/instance-id=${INSTANCEID}" \
  "cluster.autoscaler.nodegroup/instance-name=${MASTERKUBE}" \
  "cluster.autoscaler.nodegroup/node-index=0" \
  "cluster.autoscaler.nodegroup/autoprovision=false" \
  "cluster-autoscaler.kubernetes.io/scale-down-disabled=true" \
  --overwrite

kubectl label nodes ${NODENAME} "cluster.autoscaler.nodegroup/name=${NODEGROUP_NAME}" "master=true" --overwrite

echo "Done k8s master node"
