package server

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
)

// AutoScalerServerNodeState VM state
type AutoScalerServerNodeState int32

// autoScalerServerNodeStateString strings
var autoScalerServerNodeStateString = []string{
	"AutoScalerServerNodeStateNotCreated",
	"AutoScalerServerNodeStateRunning",
	"AutoScalerServerNodeStateStopped",
	"AutoScalerServerNodeStateDeleted",
	"AutoScalerServerNodeStateUndefined",
}

const (
	// AutoScalerServerNodeStateNotCreated not created state
	AutoScalerServerNodeStateNotCreated AutoScalerServerNodeState = 0

	// AutoScalerServerNodeStateRunning running state
	AutoScalerServerNodeStateRunning AutoScalerServerNodeState = 1

	// AutoScalerServerNodeStateStopped stopped state
	AutoScalerServerNodeStateStopped AutoScalerServerNodeState = 2

	// AutoScalerServerNodeStateDeleted deleted state
	AutoScalerServerNodeStateDeleted AutoScalerServerNodeState = 3

	// AutoScalerServerNodeStateUndefined undefined state
	AutoScalerServerNodeStateUndefined AutoScalerServerNodeState = 4
)

// AutoScalerServerNode Describe a AutoScaler VM
type AutoScalerServerNode struct {
	ProviderID  string `json:"providerID"`
	NodeGroupID string `json:"group"`
	// Node name and instance name could be differ when using AWS cloud provider
	InstanceName     string                    `json:"instance-name"`
	NodeName         string                    `json:"node-name"`
	NodeIndex        int                       `json:"index"`
	InstanceType     string                    `json:"instance-Type"`
	Disk             int                       `json:"disk"`
	Addresses        []string                  `json:"addresses"`
	State            AutoScalerServerNodeState `json:"state"`
	AutoProvisionned bool                      `json:"auto"`
	AwsConfig        *aws.Configuration        `json:"aws"`
	RunningInstance  *aws.Ec2Instance
	serverConfig     *types.AutoScalerServerConfig
}

func (s AutoScalerServerNodeState) String() string {
	return autoScalerServerNodeStateString[s]
}

func (vm *AutoScalerServerNode) kubeAdmJoin() error {
	kubeAdm := vm.serverConfig.KubeAdm

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
	}

	// Append extras arguments
	if len(kubeAdm.ExtraArguments) > 0 {
		args = append(args, kubeAdm.ExtraArguments...)
	}

	if output, err := utils.Sudo(vm.serverConfig.SSH, vm.Addresses[0], vm.AwsConfig.Timeout, strings.Join(args, " ")); err != nil {
		glog.Errorf("Kubeadm join %s %s return an error: %s", vm.InstanceName, vm.NodeGroupID, output)
		return err
	}

	return nil
}

func (vm *AutoScalerServerNode) setNodeLabels(c types.ClientGenerator, nodeLabels, systemLabels KubernetesLabel) error {
	labels := map[string]string{
		constantes.NodeLabelGroupName: vm.NodeGroupID,
	}

	// Append extras arguments
	for k, v := range nodeLabels {
		labels[k] = v
	}

	for k, v := range systemLabels {
		labels[k] = v
	}

	if err := c.LabelNode(vm.NodeName, labels); err != nil {
		return fmt.Errorf(constantes.ErrLabelNodeReturnError, vm.NodeName, err)
	}

	annotations := map[string]string{
		constantes.NodeLabelGroupName:             vm.NodeGroupID,
		constantes.AnnotationInstanceName:         vm.InstanceName,
		constantes.AnnotationInstanceID:           *vm.RunningInstance.InstanceID,
		constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(vm.AutoProvisionned),
		constantes.AnnotationNodeIndex:            strconv.Itoa(vm.NodeIndex),
	}

	if err := c.AnnoteNode(vm.NodeName, annotations); err != nil {
		return fmt.Errorf(constantes.ErrAnnoteNodeReturnError, vm.NodeName, err)
	}

	return nil
}

// CheckIfIPIsReady method SSH test IP
func (vm *AutoScalerServerNode) CheckIfIPIsReady(nodename, address string) error {
	var err error

	// Set hostname
	if _, err = utils.Sudo(vm.serverConfig.SSH, address, 1, fmt.Sprintf("hostnamectl set-hostname %s", nodename)); err != nil {
		return err
	}

	// Node name and instance name could be differ when using AWS cloud provider
	if vm.serverConfig.CloudProvider == "aws" {
		var nodeName string

		if nodeName, err = utils.Sudo(vm.serverConfig.SSH, address, 1, "curl -s http://169.254.169.254/latest/meta-data/local-hostname"); err != nil {
			return err
		}

		vm.NodeName = nodeName

		glog.Debugf("Launch VM:%s set to nodeName: %s", nodename, nodeName)
	}

	return nil
}

func (vm *AutoScalerServerNode) registerDNS(address string) error {
	var err error

	aws := vm.AwsConfig

	if aws.Network.ZoneID != nil {
		err = vm.RunningInstance.RegisterDNS(*aws.Network.ZoneID, fmt.Sprintf("%s.%s", vm.InstanceName, *aws.Network.PrivateZoneName), address, false)
	}

	return err
}

func (vm *AutoScalerServerNode) unregisterDNS(address string) error {
	var err error

	aws := vm.AwsConfig

	if aws.Network.ZoneID != nil {
		err = vm.RunningInstance.UnRegisterDNS(*aws.Network.ZoneID, fmt.Sprintf("%s.%s", vm.InstanceName, *aws.Network.PrivateZoneName), address, false)
	}

	return err
}

func (vm *AutoScalerServerNode) kubeletDefault() *string {
	var maxPods = vm.serverConfig.MaxPods
	var kubeletExtraArgs string

	if maxPods == 0 {
		maxPods = 110
	}

	if vm.serverConfig.CloudProvider == "aws" {
		kubeletExtraArgs = fmt.Sprintf("KUBELET_EXTRA_ARGS=\\\"$KUBELET_EXTRA_ARGS --cloud-provider=aws --max-pods=%d --node-ip=$LOCAL_IP --provider-id=%s\\\"", maxPods, vm.ProviderID)
	} else {
		kubeletExtraArgs = fmt.Sprintf("KUBELET_EXTRA_ARGS=\\\"$KUBELET_EXTRA_ARGS --max-pods=%d --node-ip=$LOCAL_IP --provider-id=%s\\\"", maxPods, vm.ProviderID)
	}

	kubeletDefault := []string{
		"#!/bin/bash",
		"source /etc/default/kubelet",
		"LOCAL_IP=$(curl http://169.254.169.254/latest/meta-data/local-ipv4)",
		"echo \"" + kubeletExtraArgs + "\" > /etc/default/kubelet",
		"systemctl restart kubelet",
	}

	result := base64.StdEncoding.EncodeToString([]byte(strings.Join(kubeletDefault, "\n")))

	return &result
}

func (vm *AutoScalerServerNode) launchVM(c types.ClientGenerator, nodeLabels, systemLabels KubernetesLabel) error {
	glog.Debugf("AutoScalerNode::launchVM, node:%s", vm.InstanceName)

	var err error
	var status AutoScalerServerNodeState
	var address *string

	aws := vm.AwsConfig

	glog.Infof("Launch VM:%s for nodegroup: %s", vm.InstanceName, vm.NodeGroupID)

	if !vm.AutoProvisionned {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if vm.State != AutoScalerServerNodeStateNotCreated {

		err = fmt.Errorf(constantes.ErrVMAlreadyCreated, vm.InstanceName)

	} else if vm.RunningInstance, err = aws.Create(vm.NodeIndex, vm.NodeGroupID, vm.InstanceName, vm.InstanceType, vm.Disk, vm.kubeletDefault()); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToLaunchVM, vm.InstanceName, err)

	} else if address, err = vm.RunningInstance.WaitForIP(vm); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if err = vm.registerDNS(*address); err != nil {

		err = fmt.Errorf(constantes.ErrRegisterDNSVMFailed, vm.InstanceName, err)

	} else if status, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrGetVMInfoFailed, vm.InstanceName, err)

	} else if status != AutoScalerServerNodeStateRunning {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if err = vm.kubeAdmJoin(); err != nil {

		err = fmt.Errorf(constantes.ErrKubeAdmJoinFailed, vm.InstanceName, err)

	} else if err = c.SetProviderID(vm.NodeName, vm.ProviderID); err != nil {

		err = fmt.Errorf(constantes.ErrProviderIDNotConfigured, vm.NodeName, err)

	} else if err = c.WaitNodeToBeReady(vm.NodeName, int(aws.Timeout)); err != nil {

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

	if !vm.AutoProvisionned || vm.RunningInstance == nil {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if state == AutoScalerServerNodeStateStopped {

		if err = vm.RunningInstance.PowerOn(); err != nil {

			err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

		} else if _, err = vm.RunningInstance.WaitForIP(vm); err != nil {

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

	if !vm.AutoProvisionned || vm.RunningInstance == nil {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStopVMFailed, vm.InstanceName, err)

	} else if state == AutoScalerServerNodeStateRunning {

		if err = c.CordonNode(vm.NodeName); err != nil {
			glog.Errorf(constantes.ErrCordonNodeReturnError, vm.NodeName, err)
		}

		if err = vm.RunningInstance.PowerOff(); err == nil {
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

	if !vm.AutoProvisionned || vm.RunningInstance == nil {
		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)
	} else {
		if status, err = vm.RunningInstance.Status(); err == nil {
			if err = vm.unregisterDNS(status.Address); err != nil {
				glog.Warnf("unable to unregister DNS entry, reason: %v", err)
			}

			if status.Powered {
				if err = c.MarkDrainNode(vm.NodeName); err != nil {
					glog.Errorf(constantes.ErrCordonNodeReturnError, vm.NodeName, err)
				}

				if err = c.DrainNode(vm.NodeName, true, true); err != nil {
					glog.Errorf(constantes.ErrDrainNodeReturnError, vm.NodeName, err)
				}

				if err = c.DeleteNode(vm.NodeName); err != nil {
					glog.Errorf(constantes.ErrDeleteNodeReturnError, vm.NodeName, err)
				}

				if err = vm.RunningInstance.PowerOff(); err != nil {
					err = fmt.Errorf(constantes.ErrStopVMFailed, vm.InstanceName, err)
				} else {
					vm.State = AutoScalerServerNodeStateStopped

					if err = vm.RunningInstance.Delete(); err != nil {
						err = fmt.Errorf(constantes.ErrDeleteVMFailed, vm.InstanceName, err)
					}
				}
			} else if err = vm.RunningInstance.Delete(); err != nil {
				err = fmt.Errorf(constantes.ErrDeleteVMFailed, vm.InstanceName, err)
			}
		}
	}

	if err == nil {
		glog.Infof("Deleted VM:%s", vm.InstanceName)
		vm.State = AutoScalerServerNodeStateDeleted
	} else {
		glog.Errorf("Could not delete VM:%s. Reason: %s", vm.InstanceName, err)
	}

	return err
}

func (vm *AutoScalerServerNode) statusVM() (AutoScalerServerNodeState, error) {
	glog.Debugf("AutoScalerNode::statusVM, node:%s", vm.InstanceName)

	// Get VM infos
	var status *aws.Status
	var err error

	if vm.RunningInstance == nil {
		return AutoScalerServerNodeStateNotCreated, nil
	}

	if status, err = vm.RunningInstance.Status(); err != nil {
		glog.Errorf(constantes.ErrGetVMInfoFailed, vm.InstanceName, err)
		return AutoScalerServerNodeStateUndefined, err
	}

	if status != nil {
		vm.Addresses = []string{
			status.Address,
		}

		if status.Powered {
			vm.State = AutoScalerServerNodeStateRunning
		} else {
			vm.State = AutoScalerServerNodeStateStopped
		}

		return vm.State, nil
	}

	return AutoScalerServerNodeStateUndefined, fmt.Errorf(constantes.ErrAutoScalerInfoNotFound, vm.InstanceName)
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
