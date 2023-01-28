package server

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uid "k8s.io/apimachinery/pkg/types"
)

// AutoScalerServerNodeState VM state
type AutoScalerServerNodeState int32

// AutoScalerServerNodeType node class (external, autoscaled, managed)
type AutoScalerServerNodeType int32

// autoScalerServerNodeStateString strings
var autoScalerServerNodeStateString = []string{
	"AutoScalerServerNodeStateNotCreated",
	"AutoScalerServerNodeStateCreating",
	"AutoScalerServerNodeStateRunning",
	"AutoScalerServerNodeStateStopped",
	"AutoScalerServerNodeStateDeleted",
	"AutoScalerServerNodeStateUndefined",
}

const (
	// AutoScalerServerNodeStateNotCreated not created state
	AutoScalerServerNodeStateNotCreated = iota

	// AutoScalerServerNodeStateCreating running state
	AutoScalerServerNodeStateCreating

	// AutoScalerServerNodeStateRunning running state
	AutoScalerServerNodeStateRunning

	// AutoScalerServerNodeStateStopped stopped state
	AutoScalerServerNodeStateStopped

	// AutoScalerServerNodeStateDeleted deleted state
	AutoScalerServerNodeStateDeleted

	// AutoScalerServerNodeStateUndefined undefined state
	AutoScalerServerNodeStateUndefined
)

const (
	// AutoScalerServerNodeExternal is a node create out of autoscaler
	AutoScalerServerNodeExternal = iota
	// AutoScalerServerNodeAutoscaled is a node create by autoscaler
	AutoScalerServerNodeAutoscaled
	// AutoScalerServerNodeManaged is a node managed by controller
	AutoScalerServerNodeManaged
)

// AutoScalerServerNode Describe a AutoScaler VM
// Node name and instance name could be differ when using AWS cloud provider
type AutoScalerServerNode struct {
	NodeGroupID      string                    `json:"group"`
	InstanceName     string                    `json:"instance-name"`
	NodeName         string                    `json:"node-name"`
	NodeIndex        int                       `json:"index"`
	CRDUID           uid.UID                   `json:"crd-uid"`
	Memory           int                       `json:"memory"`
	CPU              int                       `json:"cpu"`
	DiskSize         int                       `json:"diskSize"`
	DiskType         string                    `default:"standard" json:"diskType"`
	InstanceType     string                    `json:"instance-Type"`
	IPAddress        string                    `json:"address"`
	State            AutoScalerServerNodeState `json:"state"`
	NodeType         AutoScalerServerNodeType  `json:"type"`
	ControlPlaneNode bool                      `json:"control-plane,omitempty"`
	AllowDeployment  bool                      `json:"allow-deployment,omitempty"`
	ExtraLabels      types.KubernetesLabel     `json:"labels,omitempty"`
	ExtraAnnotations types.KubernetesLabel     `json:"annotations,omitempty"`
	awsConfig        *aws.Configuration
	runningInstance  *aws.Ec2Instance
	desiredENI       *aws.UserDefinedNetworkInterface
	serverConfig     *types.AutoScalerServerConfig
}

func (s AutoScalerServerNodeState) String() string {
	return autoScalerServerNodeStateString[s]
}

func (vm *AutoScalerServerNode) waitReady(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::waitReady, node:%s", vm.NodeName)

	return c.WaitNodeToBeReady(vm.NodeName)
}

func (vm *AutoScalerServerNode) recopyEtcdSslFilesIfNeeded() error {
	var err error

	if vm.ControlPlaneNode || *vm.serverConfig.UseExternalEtdc {
		glog.Infof("Recopy Etcd ssl files for instance: %s in node group: %s", vm.InstanceName, vm.NodeGroupID)

		if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, vm.serverConfig.ExtSourceEtcdSslDir, "."); err != nil {
			glog.Errorf("scp failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, fmt.Sprintf("mkdir -p %s", vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("mkdir failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, fmt.Sprintf("cp -r %s/* %s", filepath.Base(vm.serverConfig.ExtSourceEtcdSslDir), vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("mv failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, fmt.Sprintf("chown -R root:root %s", vm.serverConfig.ExtDestinationEtcdSslDir)); err != nil {
			glog.Errorf("chown failed: %v", err)
		}
	}

	return err
}

func (vm *AutoScalerServerNode) recopyKubernetesPKIIfNeeded() error {
	var err error

	if vm.ControlPlaneNode {
		glog.Infof("Recopy PKI for instance: %s in node group: %s", vm.InstanceName, vm.NodeGroupID)

		if err = utils.Scp(vm.serverConfig.SSH, vm.IPAddress, vm.serverConfig.KubernetesPKISourceDir, "."); err != nil {
			glog.Errorf("scp failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, fmt.Sprintf("mkdir -p %s", vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("mkdir failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, fmt.Sprintf("cp -r %s/* %s", filepath.Base(vm.serverConfig.KubernetesPKISourceDir), vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("mv failed: %v", err)
		} else if _, err = utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, fmt.Sprintf("chown -R root:root %s", vm.serverConfig.KubernetesPKIDestDir)); err != nil {
			glog.Errorf("chown failed: %v", err)
		}
	}

	return err
}

func (vm *AutoScalerServerNode) kubeAdmJoin(c types.ClientGenerator) error {
	kubeAdm := vm.serverConfig.KubeAdm

	glog.Infof("Register node in cluster for instance: %s in node group: %s", vm.InstanceName, vm.NodeGroupID)

	args := []string{
		"kubeadm",
		"join",
		kubeAdm.Address,
		"--node-name",
		vm.NodeName,
		"--token",
		kubeAdm.Token,
		"--discovery-token-ca-cert-hash",
		kubeAdm.CACert,
		"--apiserver-advertise-address",
		vm.IPAddress,
	}

	if vm.ControlPlaneNode {
		args = append(args, "--control-plane")
	}

	// Append extras arguments
	if len(kubeAdm.ExtraArguments) > 0 {
		args = append(args, kubeAdm.ExtraArguments...)
	}

	command := strings.Join(args, " ")

	if out, err := utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, command); err != nil {
		return fmt.Errorf("unable to execute command: %s, output: %s, reason:%v", command, out, err)
	}

	// To be sure, with kubeadm 1.26.1, the kubelet is not correctly restarted
	time.Sleep(5 * time.Second)

	return utils.PollImmediate(5*time.Second, time.Duration(vm.serverConfig.SSH.WaitSshReadyInSeconds)*time.Second, func() (done bool, err error) {
		if node, err := c.GetNode(vm.NodeName); err == nil && node != nil {
			return true, nil
		}

		glog.Infof("Restart kubelet for node:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

		if out, err := utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, "systemctl restart kubelet"); err != nil {
			return false, fmt.Errorf("unable to restart kubelet, output: %s, reason:%v", out, err)
		}

		return false, nil
	})
}

func (vm *AutoScalerServerNode) retrieveNodeInfo(c types.ClientGenerator) error {
	if nodeInfo, err := c.GetNode(vm.NodeName); err != nil {
		return err
	} else {
		vm.CPU = int(nodeInfo.Status.Capacity.Cpu().Value())
		vm.Memory = int(nodeInfo.Status.Capacity.Memory().Value() / (1024 * 1024))
		vm.DiskSize = int(nodeInfo.Status.Capacity.Storage().Value() / (1024 * 1024))
	}

	return nil
}

func (vm *AutoScalerServerNode) k3sAgentJoin(c types.ClientGenerator) error {
	kubeAdm := vm.serverConfig.KubeAdm
	k3s := vm.serverConfig.K3S
	args := []string{
		fmt.Sprintf("echo K3S_ARGS='--kubelet-arg=provider-id=%s --node-name=%s --server=https://%s --token=%s' > /etc/systemd/system/k3s.service.env", vm.generateProviderID(), vm.NodeName, kubeAdm.Address, kubeAdm.Token),
	}

	if vm.ControlPlaneNode {
		if vm.serverConfig.UseControllerManager != nil && *vm.serverConfig.UseControllerManager {
			args = append(args, "echo 'K3S_MODE=server' > /etc/default/k3s", "echo K3S_DISABLE_ARGS='--disable-cloud-controller --disable=servicelb --disable=traefik --disable=metrics-server' > /etc/systemd/system/k3s.disabled.env")
		} else {
			args = append(args, "echo 'K3S_MODE=server' > /etc/default/k3s", "echo K3S_DISABLE_ARGS='--disable=servicelb --disable=traefik --disable=metrics-server' > /etc/systemd/system/k3s.disabled.env")
		}
		if vm.serverConfig.UseExternalEtdc != nil && *vm.serverConfig.UseExternalEtdc {
			args = append(args, fmt.Sprintf("echo K3S_SERVER_ARGS='--datastore-endpoint=%s --datastore-cafile=%s/ca.pem --datastore-certfile=%s/etcd.pem --datastore-keyfile=%s/etcd-key.pem' > /etc/systemd/system/k3s.server.env", k3s.DatastoreEndpoint, vm.serverConfig.ExtDestinationEtcdSslDir, vm.serverConfig.ExtDestinationEtcdSslDir, vm.serverConfig.ExtDestinationEtcdSslDir))
		}
	}

	// Append extras arguments
	if len(k3s.ExtraCommands) > 0 {
		args = append(args, k3s.ExtraCommands...)
	}

	args = append(args, "systemctl enable k3s.service", "systemctl start k3s.service")

	glog.Infof("Join cluster for node:%s for nodegroup: %s", vm.NodeName, vm.NodeGroupID)

	command := fmt.Sprintf("sh -c \"%s\"", strings.Join(args, " && "))
	if out, err := utils.Sudo(vm.serverConfig.SSH, vm.IPAddress, vm.awsConfig.Timeout, command); err != nil {
		return fmt.Errorf("unable to execute command: %s, output: %s, reason:%v", command, out, err)
	}

	return utils.PollImmediate(5*time.Second, time.Duration(vm.serverConfig.SSH.WaitSshReadyInSeconds)*time.Second, func() (done bool, err error) {
		if node, err := c.GetNode(vm.NodeName); err == nil && node != nil {
			return true, nil
		}

		return false, nil
	})
}

func (vm *AutoScalerServerNode) joinCluster(c types.ClientGenerator) error {
	if vm.serverConfig.UseK3S != nil && *vm.serverConfig.UseK3S {
		return vm.k3sAgentJoin(c)
	} else {
		return vm.kubeAdmJoin(c)
	}
}

func (vm *AutoScalerServerNode) setNodeLabels(c types.ClientGenerator, nodeLabels, systemLabels types.KubernetesLabel) error {
	topology := types.KubernetesLabel{
		constantes.NodeLabelTopologyRegion: *vm.runningInstance.Region,
		constantes.NodeLabelTopologyZone:   *vm.runningInstance.Zone,
	}

	labels := utils.MergeKubernetesLabel(nodeLabels, topology, systemLabels, vm.ExtraLabels)

	if err := c.LabelNode(vm.NodeName, labels); err != nil {
		return fmt.Errorf(constantes.ErrLabelNodeReturnError, vm.NodeName, err)
	}

	annotations := types.KubernetesLabel{
		constantes.AnnotationNodeGroupName:        vm.NodeGroupID,
		constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(vm.NodeType != AutoScalerServerNodeAutoscaled),
		constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(vm.NodeType == AutoScalerServerNodeAutoscaled),
		constantes.AnnotationNodeManaged:          strconv.FormatBool(vm.NodeType == AutoScalerServerNodeManaged),
		constantes.AnnotationNodeIndex:            strconv.Itoa(vm.NodeIndex),
		constantes.AnnotationInstanceName:         vm.InstanceName,
		constantes.AnnotationInstanceID:           *vm.runningInstance.InstanceID,
	}

	annotations = utils.MergeKubernetesLabel(annotations, vm.ExtraAnnotations)

	if err := c.AnnoteNode(vm.NodeName, annotations); err != nil {
		return fmt.Errorf(constantes.ErrAnnoteNodeReturnError, vm.NodeName, err)
	}

	if vm.ControlPlaneNode && vm.AllowDeployment {
		if err := c.TaintNode(vm.NodeName,
			apiv1.Taint{
				Key:    constantes.NodeLabelControlPlaneRole,
				Effect: apiv1.TaintEffectNoSchedule,
				TimeAdded: &metav1.Time{
					Time: time.Now(),
				},
			},
			apiv1.Taint{
				Key:    constantes.NodeLabelMasterRole,
				Effect: apiv1.TaintEffectNoSchedule,
				TimeAdded: &metav1.Time{
					Time: time.Now(),
				},
			}); err != nil {
			return fmt.Errorf(constantes.ErrTaintNodeReturnError, vm.NodeName, err)
		}
	}

	return nil
}

// WaitSSHReady method SSH test IP
func (vm *AutoScalerServerNode) WaitSSHReady(nodename, address string) error {
	return utils.PollImmediate(time.Second, time.Duration(vm.serverConfig.SSH.WaitSshReadyInSeconds)*time.Second, func() (bool, error) {
		// Set hostname
		if _, err := utils.Sudo(vm.serverConfig.SSH, address, time.Second, fmt.Sprintf("hostnamectl set-hostname %s", nodename)); err != nil {
			if strings.HasSuffix(err.Error(), "connection refused") || strings.HasSuffix(err.Error(), "i/o timeout") {
				return false, nil
			}

			return false, err
		}

		// Node name and instance name could be differ when using AWS cloud provider
		if vm.serverConfig.CloudProvider == "aws" {

			if nodeName, err := utils.Sudo(vm.serverConfig.SSH, address, 1, "curl -s http://169.254.169.254/latest/meta-data/local-hostname"); err == nil {
				vm.NodeName = nodeName

				glog.Debugf("Launch VM:%s set to nodeName: %s", nodename, nodeName)
			} else {
				return false, err
			}
		}

		return true, nil
	})

}

func (vm *AutoScalerServerNode) WaitForIP() (*string, error) {
	glog.Infof("Wait IP ready for instance: %s in node group: %s", vm.InstanceName, vm.NodeGroupID)

	return vm.runningInstance.WaitForIP(vm)
}

func (vm *AutoScalerServerNode) registerDNS(address string) error {
	var err error

	aws := vm.awsConfig

	if len(aws.Network.ZoneID) > 0 {
		hostname := fmt.Sprintf("%s.%s", vm.InstanceName, aws.Network.PrivateZoneName)

		glog.Infof("Register route53 entry for instance %s, node group: %s, hostname: %s with IP:%s", vm.InstanceName, vm.NodeGroupID, hostname, address)

		err = vm.runningInstance.RegisterDNS(aws, hostname, address, vm.serverConfig.SSH.TestMode)
	}

	return err
}

func (vm *AutoScalerServerNode) unregisterDNS(address string) error {
	var err error

	aws := vm.awsConfig

	if len(aws.Network.ZoneID) > 0 {
		hostname := fmt.Sprintf("%s.%s", vm.InstanceName, aws.Network.PrivateZoneName)

		glog.Infof("Unregister route53 entry for instance %s, node group: %s, hostname: %s with IP:%s", vm.InstanceName, vm.NodeGroupID, hostname, address)

		err = vm.runningInstance.UnRegisterDNS(aws, hostname, false)
	}

	return err
}

func (vm *AutoScalerServerNode) kubeletDefault() *string {
	var maxPods = vm.serverConfig.MaxPods
	var kubeletExtraArgs string
	var cloudProvider string

	if maxPods == 0 {
		maxPods = 110
	}

	if len(vm.serverConfig.CloudProvider) > 0 {
		cloudProvider = fmt.Sprintf("--cloud-provider=%s", vm.serverConfig.CloudProvider)
	}

	kubeletExtraArgs = fmt.Sprintf("KUBELET_EXTRA_ARGS=\\\"$KUBELET_EXTRA_ARGS --max-pods=%d --node-ip=$LOCAL_IP --provider-id=aws://$ZONEID/$INSTANCEID %s\\\"", maxPods, cloudProvider)

	kubeletDefault := []string{
		"#!/bin/bash",
		"source /etc/default/kubelet",
		"INSTANCEID=$(curl http://169.254.169.254/latest/meta-data/instance-id)",
		"ZONEID=$(curl http://169.254.169.254/latest/meta-data/placement/availability-zone)",
		"LOCAL_IP=$(curl http://169.254.169.254/latest/meta-data/local-ipv4)",
		"echo \"" + kubeletExtraArgs + "\" > /etc/default/kubelet",
		"systemctl restart kubelet",
	}

	result := base64.StdEncoding.EncodeToString([]byte(strings.Join(kubeletDefault, "\n")))

	return &result
}

func (vm *AutoScalerServerNode) launchVM(c types.ClientGenerator, nodeLabels, systemLabels types.KubernetesLabel) error {
	glog.Debugf("AutoScalerNode::launchVM, node:%s", vm.InstanceName)

	var err error
	var status AutoScalerServerNodeState
	var address *string

	aws := vm.awsConfig

	glog.Infof("Launch VM:%s for nodegroup: %s", vm.InstanceName, vm.NodeGroupID)

	if vm.State != AutoScalerServerNodeStateNotCreated {
		return fmt.Errorf(constantes.ErrVMAlreadyCreated, vm.NodeName)
	}

	if aws.Exists(vm.NodeName) {
		glog.Warnf(constantes.ErrVMAlreadyExists, vm.NodeName)
		return fmt.Errorf(constantes.ErrVMAlreadyExists, vm.NodeName)
	}

	vm.State = AutoScalerServerNodeStateCreating

	if vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if vm.runningInstance, err = aws.Create(vm.NodeIndex, vm.NodeGroupID, vm.InstanceName, vm.InstanceType, vm.DiskType, vm.DiskSize, vm.kubeletDefault(), vm.desiredENI); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToLaunchVM, vm.InstanceName, err)

	} else if address, err = vm.WaitForIP(); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if err = vm.registerDNS(*address); err != nil {

		err = fmt.Errorf(constantes.ErrRegisterDNSVMFailed, vm.InstanceName, err)

	} else if status, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrGetVMInfoFailed, vm.InstanceName, err)

	} else if status != AutoScalerServerNodeStateRunning {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if err = vm.recopyKubernetesPKIIfNeeded(); err != nil {

		err = fmt.Errorf(constantes.ErrRecopyKubernetesPKIFailed, vm.NodeName, err)

	} else if err = vm.recopyEtcdSslFilesIfNeeded(); err != nil {

		err = fmt.Errorf(constantes.ErrUpdateEtcdSslFailed, vm.NodeName, err)

	} else if err = vm.joinCluster(c); err != nil {

		err = fmt.Errorf(constantes.ErrKubeAdmJoinFailed, vm.InstanceName, err)

	} else if err = vm.setProviderID(c); err != nil {

		err = fmt.Errorf(constantes.ErrProviderIDNotConfigured, vm.NodeName, err)

	} else if err = vm.waitReady(c); err != nil {

		err = fmt.Errorf(constantes.ErrNodeIsNotReady, vm.InstanceName)

	} else if err = vm.retrieveNodeInfo(c); err != nil {

		err = fmt.Errorf(constantes.ErrNodeIsNotReady, vm.InstanceName)

	} else {
		err = vm.setNodeLabels(c, nodeLabels, systemLabels)
	}

	if err == nil {
		glog.Infof("Launched VM:%s nodename:%s for nodegroup: %s", vm.InstanceName, vm.NodeName, vm.NodeGroupID)
	} else {
		glog.Errorf("Unable to launch VM:%s for nodegroup: %s. Reason: %v", vm.InstanceName, vm.NodeGroupID, err.Error())
	}

	return err
}

func (vm *AutoScalerServerNode) startVM(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::startVM, node:%s", vm.InstanceName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Start VM:%s", vm.InstanceName)

	if (vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged) || vm.runningInstance == nil {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if state == AutoScalerServerNodeStateStopped {

		if err = vm.runningInstance.PowerOn(); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

		} else if _, err = vm.runningInstance.WaitForIP(vm); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

		} else if state, err = vm.statusVM(); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

		} else if state != AutoScalerServerNodeStateRunning {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

		} else {
			if err = c.UncordonNode(vm.NodeName); err != nil {
				glog.Errorf(constantes.ErrUncordonNodeReturnError, vm.NodeName, err)

				err = nil
			}

			vm.State = AutoScalerServerNodeStateRunning
		}

	} else if state != AutoScalerServerNodeStateRunning {
		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, fmt.Sprintf("Unexpected state: %d", state))
	}

	if err == nil {
		glog.Infof("Started VM:%s", vm.InstanceName)
	} else {
		glog.Errorf("Unable to start VM:%s. Reason: %v", vm.InstanceName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) stopVM(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::stopVM, node:%s", vm.InstanceName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Stop VM:%s", vm.InstanceName)

	if (vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged) || vm.runningInstance == nil {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStopVMFailed, vm.InstanceName, err)

	} else if state == AutoScalerServerNodeStateRunning {

		if err = c.CordonNode(vm.NodeName); err != nil {
			glog.Errorf(constantes.ErrCordonNodeReturnError, vm.NodeName, err)
		}

		if err = vm.runningInstance.PowerOff(); err == nil {
			vm.State = AutoScalerServerNodeStateStopped
		} else {
			err = fmt.Errorf(constantes.ErrStopVMFailed, vm.InstanceName, err)
		}

	} else if state != AutoScalerServerNodeStateStopped {

		err = fmt.Errorf(constantes.ErrStopVMFailed, vm.InstanceName, fmt.Sprintf("Unexpected state: %d", state))

	}

	if err == nil {
		glog.Infof("Stopped VM:%s", vm.InstanceName)
	} else {
		glog.Errorf("Could not stop VM:%s. Reason: %s", vm.InstanceName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) deleteVM(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerNode::deleteVM, node:%s", vm.InstanceName)

	var err error
	var status *aws.Status

	if (vm.NodeType != AutoScalerServerNodeAutoscaled && vm.NodeType != AutoScalerServerNodeManaged) || vm.runningInstance == nil {
		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)
	} else {
		if status, err = vm.runningInstance.Status(); err == nil {
			if err = vm.unregisterDNS(status.Address); err != nil {
				glog.Warnf("unable to unregister DNS entry, reason: %v", err)
			}
			if status.Powered {
				// Delete kubernetes node only is alive
				if _, err = c.GetNode(vm.NodeName); err == nil {
					if err = c.MarkDrainNode(vm.NodeName); err != nil {
						glog.Errorf(constantes.ErrCordonNodeReturnError, vm.NodeName, err)
					}

					if err = c.DrainNode(vm.NodeName, true, true); err != nil {
						glog.Errorf(constantes.ErrDrainNodeReturnError, vm.NodeName, err)
					}

					if err = c.DeleteNode(vm.NodeName); err != nil {
						glog.Errorf(constantes.ErrDeleteNodeReturnError, vm.NodeName, err)
					}
				}

				if err = vm.runningInstance.PowerOff(); err != nil {
					err = fmt.Errorf(constantes.ErrStopVMFailed, vm.InstanceName, err)
				} else {
					vm.State = AutoScalerServerNodeStateStopped

					if err = vm.runningInstance.Delete(); err != nil {
						err = fmt.Errorf(constantes.ErrDeleteVMFailed, vm.InstanceName, err)
					}
				}
			} else if err = vm.runningInstance.Delete(); err != nil {
				err = fmt.Errorf(constantes.ErrDeleteVMFailed, vm.InstanceName, err)
			}
		}
	}

	if err == nil {
		glog.Infof("Deleted VM:%s", vm.InstanceName)
		vm.State = AutoScalerServerNodeStateDeleted
	} else if !strings.HasPrefix(err.Error(), "InvalidInstanceID.NotFound: The instance ID") {
		glog.Errorf("Could not delete VM:%s. Reason: %s", vm.InstanceName, err)
	} else {
		glog.Warnf("Could not delete VM:%s. does not exist", vm.InstanceName)
		err = fmt.Errorf(constantes.ErrVMNotFound, vm.InstanceName)
	}

	return err
}

func (vm *AutoScalerServerNode) statusVM() (AutoScalerServerNodeState, error) {
	glog.Debugf("AutoScalerNode::statusVM, node:%s", vm.InstanceName)

	// Get VM infos
	var status *aws.Status
	var err error

	if vm.runningInstance == nil {
		return AutoScalerServerNodeStateNotCreated, nil
	}

	if status, err = vm.runningInstance.Status(); err != nil {
		glog.Errorf(constantes.ErrGetVMInfoFailed, vm.InstanceName, err)
		return AutoScalerServerNodeStateUndefined, err
	}

	if status != nil {
		vm.IPAddress = status.Address

		if status.Powered {
			vm.State = AutoScalerServerNodeStateRunning
		} else {
			vm.State = AutoScalerServerNodeStateStopped
		}

		return vm.State, nil
	}

	return AutoScalerServerNodeStateUndefined, fmt.Errorf(constantes.ErrAutoScalerInfoNotFound, vm.InstanceName)
}

func (vm *AutoScalerServerNode) setProviderID(c types.ClientGenerator) error {
	if vm.serverConfig.UseControllerManager != nil && !*vm.serverConfig.UseControllerManager {
		return c.SetProviderID(vm.NodeName, vm.generateProviderID())
	}

	return nil
}

func (vm *AutoScalerServerNode) generateProviderID() string {
	return fmt.Sprintf("aws://%s/%s", *vm.runningInstance.Zone, *vm.runningInstance.InstanceID)
}

func (vm *AutoScalerServerNode) setServerConfiguration(config *types.AutoScalerServerConfig) {
	vm.serverConfig = config
}

// cleanOnLaunchError called when error occurs during launch
func (vm *AutoScalerServerNode) cleanOnLaunchError(c types.ClientGenerator, err error) {
	glog.Errorf(constantes.ErrUnableToLaunchVM, vm.InstanceName, err)

	if status, _ := vm.statusVM(); status != AutoScalerServerNodeStateNotCreated {
		if e := vm.deleteVM(c); e != nil {
			glog.Errorf(constantes.ErrUnableToDeleteVM, vm.InstanceName, e)
		}
	} else {
		glog.Warningf(constantes.WarnFailedVMNotDeleted, vm.InstanceName, status)
	}

}
