#!/bin/bash

set -e

CNI=flannel
NET_IF=$(ip route get 1|awk '{print $5;exit}')
KUBERNETES_VERSION=v1.16.9
CLUSTER_DIR=/etc/cluster
SCHEME="aws"
NODEGROUP_NAME="aws-ca-k8s"
MASTERKUBE="${NODEGROUP_NAME}-masterkube"
PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${HOSTNAME}"
CERT_EXTRA_SANS=

[ -z "$2" ] || NET_IF="$2"

TEMP=$(getopt -o c:p:n:k:s:i: --long cert-extra-sans:,provider-id:,cni:,cni-version:,kubernetes-version:,net-if: -n "$0" -- "$@")

eval set -- "${TEMP}"

# extract options and their arguments into variables.
while true; do
    case "$1" in
    -p | --provider-id)
        PROVIDERID="${SCHEME}://${NODEGROUP_NAME}/object?type=node&name=${MASTERKUBE}"
        shift 2
        ;;

    -i | --cni-version)
        CNI_VERSION="$2"
        shift 2
        ;;

    -n | --net-if)
        NET_IF="$2"
        shift 2
        ;;

    -c | --cni)
        CNI="$2"
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

# Check if interface exists, else take inet default gateway
ifconfig $NET_IF &> /dev/null || NET_IF=$(ip route get 1|awk '{print $5;exit}')
IPADDR=$(ip addr show $NET_IF | grep "inet\s" | tr '/' ' ' | awk '{print $2}')

mkdir -p $CLUSTER_DIR

echo -n "$IPADDR:6443" > $CLUSTER_DIR/manager-ip

sed -i "2i${IPADDR} $(hostname)" /etc/hosts

if [ -z $CERT_EXTRA_SANS ]; then
    CERT_EXTRA_SANS="--apiserver-cert-extra-sans ${HOSTNAME}"
else
    CERT_EXTRA_SANS="--apiserver-cert-extra-sans ${HOSTNAME},${CERT_EXTRA_SANS}"
fi

if [ "x$KUBERNETES_VERSION" != "x" ]; then
    K8_OPTIONS="--token-ttl 0 --ignore-preflight-errors=All --apiserver-advertise-address $IPADDR --kubernetes-version $KUBERNETES_VERSION ${CERT_EXTRA_SANS}"
else
    K8_OPTIONS="--token-ttl 0 --ignore-preflight-errors=All --apiserver-advertise-address $IPADDR ${CERT_EXTRA_SANS}"
fi

if [ ! -f /etc/kubernetes/kubelet.conf ]; then

    if [ -z "$(grep 'provider-id' /etc/default/kubelet)" ]; then
        echo "KUBELET_EXTRA_ARGS='--fail-swap-on=false --read-only-port=10255 --feature-gates=VolumeSubpathEnvExpansion=true --provider-id=${PROVIDERID}'" > /etc/default/kubelet
        systemctl restart kubelet
    fi

    if [ -z "$CNI" ]; then
        CNI="calico"
    fi

    CNI=$(echo "$CNI" | tr '[:upper:]' '[:lower:]')

    export KUBECONFIG=/etc/kubernetes/admin.conf

    if [ "$CNI" = "calico" ]; then

        K8_OPTIONS="$K8_OPTIONS --service-cidr 10.96.0.0/12 --pod-network-cidr 192.168.0.0/16"

        echo "Download calicoctl"

        curl -s -O -L https://github.com/projectcalico/calicoctl/releases/download/v3.1.0/calicoctl
        chmod +x calicoctl
        mv calicoctl /usr/local/bin

    elif [ "$CNI" = "flannel" ]; then

        [ -e /proc/sys/net/bridge/bridge-nf-call-iptables ] && sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr 10.244.0.0/16"

    elif [ "$CNI" = "weave" ]; then

        [ -e /proc/sys/net/bridge/bridge-nf-call-iptables ] && sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

    elif [ "$CNI" = "canal" ]; then

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"

    elif [ "$CNI" = "canal" ]; then

        [ -e /proc/sys/net/bridge/bridge-nf-call-iptables ] && sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"

    elif [ "$CNI" = "kube" ]; then

        [ -e /proc/sys/net/bridge/bridge-nf-call-iptables ] && sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

        K8_OPTIONS="$K8_OPTIONS --pod-network-cidr=10.244.0.0/16"

    elif [ "$CNI" = "romana" ]; then

        [ -e /proc/sys/net/bridge/bridge-nf-call-iptables ] && sysctl net.bridge.bridge-nf-call-iptables=1
        echo "net.bridge.bridge-nf-call-iptables = 1" >> /etc/sysctl.conf

    else
        echo "CNI $CNI is not supported"

        exit -1
    fi

    echo "Init K8 cluster with options:$K8_OPTIONS, PROVIDERID=${PROVIDERID}"

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

    if [ "$CNI" = "calico" ]; then

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
else
    echo "Already installed k8s master node"
fi
