package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
	"github.com/golang/glog"
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

func (vm *AutoScalerServerNode) prepareKubelet() (string, error) {
	var out string
	var err error
	var fName = fmt.Sprintf("/tmp/set-kubelet-default-%s.sh", vm.InstanceName)
	var address = vm.Addresses[0]
	var maxPods = vm.serverConfig.MaxPods
	var kubeletExtraArgs string

	if maxPods == 0 {
		maxPods = 110
	}

	if vm.serverConfig.CloudProvider == "aws" {
		kubeletExtraArgs = fmt.Sprintf("echo \"KUBELET_EXTRA_ARGS=\\\"$KUBELET_EXTRA_ARGS --cloud-provider=aws --max-pods=%d --node-ip=%s --provider-id=%s\\\"\" > /etc/default/kubelet", maxPods, address, vm.ProviderID)
	} else {
		kubeletExtraArgs = fmt.Sprintf("echo \"KUBELET_EXTRA_ARGS=\\\"$KUBELET_EXTRA_ARGS --max-pods=%d --node-ip=%s --provider-id=%s\\\"\" > /etc/default/kubelet", maxPods, address, vm.ProviderID)
	}

	kubeletDefault := []string{
		"#!/bin/bash",
		". /etc/default/kubelet",
		kubeletExtraArgs,
		"systemctl restart kubelet",
	}

	if err = ioutil.WriteFile(fName, []byte(strings.Join(kubeletDefault, "\n")), 0755); err != nil {
		return out, err
	}

	defer os.Remove(fName)

	if err = utils.Scp(vm.serverConfig.SSH, address, fName, fName); err != nil {
		glog.Errorf("Unable to scp node %s address:%s, reason:%s", address, vm.InstanceName, err)

		return out, err
	}

	if out, err = utils.Sudo(vm.serverConfig.SSH, address, vm.AwsConfig.Timeout, fmt.Sprintf("bash %s", fName)); err != nil {
		glog.Errorf("Unable to ssh node %s address:%s, reason:%s", address, vm.InstanceName, err)

		return out, err
	}

	return "", nil
}

func (vm *AutoScalerServerNode) waitReady() error {
	glog.V(5).Infof("AutoScalerNode::waitReady, node:%s", vm.InstanceName)

	kubeconfig := vm.serverConfig.KubeConfig

	// Max 60s
	for index := 0; index < 12; index++ {
		var out string
		var err error
		var arg = []string{
			"kubectl",
			"get",
			"nodes",
			vm.NodeName,
			"--output",
			"json",
			"--kubeconfig",
			kubeconfig,
		}

		if out, err = utils.Pipe(arg...); err != nil {
			return err
		}

		var nodeInfo apiv1.Node

		if err = json.Unmarshal([]byte(out), &nodeInfo); err != nil {
			return fmt.Errorf(constantes.ErrUnmarshallingError, vm.InstanceName, err)
		}

		for _, status := range nodeInfo.Status.Conditions {
			if status.Type == "Ready" {
				if b, e := strconv.ParseBool(string(status.Status)); e == nil {
					if b {
						glog.Infof("The kubernetes node %s is Ready", vm.InstanceName)
						return nil
					}
				}
			}
		}

		glog.Infof("The kubernetes node:%s is not ready", vm.InstanceName)

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf(constantes.ErrNodeIsNotReady, vm.InstanceName)
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

func (vm *AutoScalerServerNode) setNodeLabels(nodeLabels, systemLabels KubernetesLabel) error {
	if len(nodeLabels)+len(systemLabels) > 0 {

		args := []string{
			"kubectl",
			"label",
			"nodes",
			vm.NodeName,
			"node-role.kubernetes.io/worker=worker",
		}

		// Append extras arguments
		for k, v := range nodeLabels {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}

		for k, v := range systemLabels {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}

		args = append(args, "--kubeconfig")
		args = append(args, vm.serverConfig.KubeConfig)

		if out, err := utils.Pipe(args...); err != nil {
			return fmt.Errorf(constantes.ErrKubeCtlReturnError, vm.InstanceName, out, err)
		}
	}

	args := []string{
		"kubectl",
		"annotate",
		"node",
		vm.NodeName,
		fmt.Sprintf("%s=%s", constantes.NodeLabelGroupName, vm.NodeGroupID),
		fmt.Sprintf("%s=%s", constantes.AnnotationInstanceName, vm.InstanceName),
		fmt.Sprintf("%s=%s", constantes.AnnotationInstanceID, *vm.RunningInstance.InstanceID),
		fmt.Sprintf("%s=%s", constantes.AnnotationNodeAutoProvisionned, strconv.FormatBool(vm.AutoProvisionned)),
		fmt.Sprintf("%s=%d", constantes.AnnotationNodeIndex, vm.NodeIndex),
		"--overwrite",
		"--kubeconfig",
		vm.serverConfig.KubeConfig,
	}

	if out, err := utils.Pipe(args...); err != nil {
		return fmt.Errorf(constantes.ErrKubeCtlReturnError, vm.InstanceName, out, err)
	}

	return nil
}

var phDefaultRsyncFlags = []string{
	"--verbose",
	"--archive",
	"-z",
	"--copy-links",
	"--no-owner",
	"--no-group",
	"--delete",
}

func encodeCloudInit(object interface{}) string {
	var out bytes.Buffer

	fmt.Fprintln(&out, "#cloud-init")

	b, _ := yaml.Marshal(object)

	fmt.Fprintln(&out, string(b))

	return out.String()
}

func (vm *AutoScalerServerNode) syncFolders() (string, error) {

	syncFolders := vm.serverConfig.SyncFolders

	if syncFolders != nil && len(syncFolders.Folders) > 0 {
		for _, folder := range syncFolders.Folders {
			var rsync = []string{
				"rsync",
			}

			tempFile, _ := ioutil.TempFile(os.TempDir(), "aws-rsync")

			defer tempFile.Close()

			if len(syncFolders.RsyncOptions) == 0 {
				rsync = append(rsync, phDefaultRsyncFlags...)
			} else {
				rsync = append(rsync, syncFolders.RsyncOptions...)
			}

			sshOptions := []string{
				"--rsync-path",
				"sudo rsync",
				"-e",
				fmt.Sprintf("ssh -p 22 -o LogLevel=FATAL -o ControlMaster=auto -o ControlPath=%s -o ControlPersist=10m  -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i '%s'", tempFile.Name(), syncFolders.RsyncSSHKey),
			}

			excludes := make([]string, 0, len(folder.Excludes)*2)

			for _, exclude := range folder.Excludes {
				excludes = append(excludes, "--exclude", exclude)
			}

			rsync = append(rsync, sshOptions...)
			rsync = append(rsync, excludes...)
			rsync = append(rsync, folder.Source, fmt.Sprintf("%s@%s:%s", syncFolders.RsyncUser, vm.Addresses[0], folder.Destination))

			if out, err := utils.Pipe(rsync...); err != nil {
				return out, err
			}
		}
	}
	//["/usr/bin/rsync", "--verbose", "--archive", "--delete", "-z", "--copy-links", "--no-owner", "--no-group", "--rsync-path", "sudo rsync", "-e",
	// "ssh -p 22 -o LogLevel=FATAL  -o ControlMaster=auto -o ControlPath=/tmp/vagrant-rsync-20181227-31508-1sjw4bm -o ControlPersist=10m  -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i '/home/fboltz/.ssh/id_rsa'",
	// "--exclude", ".vagrant/", "/home/fboltz/Projects/vagrant-multipass/", "vagrant@10.196.85.125:/vagrant"]
	return "", nil
}

// CheckIfIPIsReady method SSH test IP
func (vm *AutoScalerServerNode) CheckIfIPIsReady(nodename, address string) error {
	var err error

	// Set hostname
	if _, err = utils.Sudo(vm.serverConfig.SSH, address, 0.5, fmt.Sprintf("hostnamectl set-hostname %s", nodename)); err != nil {
		return err
	}

	// Node name and instance name could be differ when using AWS cloud provider
	if vm.serverConfig.CloudProvider == "aws" {
		var nodeName string

		if nodeName, err = utils.Sudo(vm.serverConfig.SSH, address, 0.5, "curl -s http://169.254.169.254/latest/meta-data/local-hostname"); err != nil {
			return err
		}

		vm.NodeName = nodeName

		glog.V(5).Infof("Launch VM:%s set to nodeName: %s", nodename, nodeName)
	}

	return nil
}

func (vm *AutoScalerServerNode) launchVM(nodeLabels, systemLabels KubernetesLabel) error {
	glog.V(5).Infof("AutoScalerNode::launchVM, node:%s", vm.InstanceName)

	var err error
	var status AutoScalerServerNodeState
	var output string

	aws := vm.AwsConfig

	glog.Infof("Launch VM:%s for nodegroup: %s", vm.InstanceName, vm.NodeGroupID)

	if vm.AutoProvisionned == false {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if vm.State != AutoScalerServerNodeStateNotCreated {

		err = fmt.Errorf(constantes.ErrVMAlreadyCreated, vm.InstanceName)

	} else if vm.RunningInstance, err = aws.Create(vm.NodeIndex, vm.NodeGroupID, vm.InstanceName, vm.InstanceType, vm.Disk, vm.serverConfig.CloudInit); err != nil {

		err = fmt.Errorf(constantes.ErrUnableToLaunchVM, vm.InstanceName, err)

	} else if _, err = vm.RunningInstance.WaitForIP(vm); err != nil {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if status, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrGetVMInfoFailed, vm.InstanceName, err)

	} else if status != AutoScalerServerNodeStateRunning {

		err = fmt.Errorf(constantes.ErrStartVMFailed, vm.InstanceName, err)

	} else if output, err = vm.syncFolders(); err != nil {

		err = fmt.Errorf(constantes.ErrRsyncError, vm.InstanceName, output, err)

	} else if output, err = vm.prepareKubelet(); err != nil {

		err = fmt.Errorf(constantes.ErrKubeletNotConfigured, vm.InstanceName, output, err)

	} else if err = vm.kubeAdmJoin(); err != nil {

		err = fmt.Errorf(constantes.ErrKubeAdmJoinFailed, vm.InstanceName, err)

	} else if err = vm.waitReady(); err != nil {

		err = fmt.Errorf(constantes.ErrNodeIsNotReady, vm.InstanceName)

	} else {
		err = vm.setNodeLabels(nodeLabels, systemLabels)
	}

	if err == nil {
		glog.Infof("Launched VM:%s nodename:%s for nodegroup: %s", vm.InstanceName, vm.NodeName, vm.NodeGroupID)
	} else {
		glog.Errorf("Unable to launch VM:%s for nodegroup: %s. Reason: %v", vm.InstanceName, vm.NodeGroupID, err.Error())
	}

	return err
}

func (vm *AutoScalerServerNode) startVM() error {
	glog.V(5).Infof("AutoScalerNode::startVM, node:%s", vm.InstanceName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Start VM:%s", vm.InstanceName)

	kubeconfig := vm.serverConfig.KubeConfig

	if vm.AutoProvisionned == false || vm.RunningInstance == nil {

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
			args := []string{
				"kubectl",
				"uncordon",
				vm.NodeName,
				"--kubeconfig",
				kubeconfig,
			}

			if err = utils.Shell(args...); err != nil {
				glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.InstanceName, err)

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

func (vm *AutoScalerServerNode) stopVM() error {
	glog.V(5).Infof("AutoScalerNode::stopVM, node:%s", vm.InstanceName)

	var err error
	var state AutoScalerServerNodeState

	glog.Infof("Stop VM:%s", vm.InstanceName)

	kubeconfig := vm.serverConfig.KubeConfig

	if vm.AutoProvisionned == false || vm.RunningInstance == nil {

		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)

	} else if state, err = vm.statusVM(); err != nil {

		err = fmt.Errorf(constantes.ErrStopVMFailed, vm.InstanceName, err)

	} else if state == AutoScalerServerNodeStateRunning {

		args := []string{
			"kubectl",
			"cordon",
			vm.NodeName,
			"--kubeconfig",
			kubeconfig,
		}

		if err = utils.Shell(args...); err != nil {
			glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.InstanceName, err)
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

func (vm *AutoScalerServerNode) deleteVM() error {
	glog.V(5).Infof("AutoScalerNode::deleteVM, node:%s", vm.InstanceName)

	var err error
	var status *aws.Status

	if vm.AutoProvisionned == false || vm.RunningInstance == nil {
		err = fmt.Errorf(constantes.ErrVMNotProvisionnedByMe, vm.InstanceName)
	} else {
		kubeconfig := vm.serverConfig.KubeConfig

		if status, err = vm.RunningInstance.Status(); err == nil {
			if status.Powered {
				args := []string{
					"kubectl",
					"drain",
					vm.NodeName,
					"--delete-local-data",
					"--force",
					"--ignore-daemonsets",
					"--kubeconfig",
					kubeconfig,
				}

				if err = utils.Shell(args...); err != nil {
					glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.InstanceName, err)
				}

				args = []string{
					"kubectl",
					"delete",
					"node",
					vm.NodeName,
					"--kubeconfig",
					kubeconfig,
				}

				if err = utils.Shell(args...); err != nil {
					glog.Errorf(constantes.ErrKubeCtlIgnoredError, vm.InstanceName, err)
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
	glog.V(5).Infof("AutoScalerNode::statusVM, node:%s", vm.InstanceName)

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
func (vm *AutoScalerServerNode) cleanOnLaunchError(err error) {
	glog.Errorf(constantes.ErrUnableToLaunchVM, vm.InstanceName, err)

	if status, _ := vm.statusVM(); status != AutoScalerServerNodeStateNotCreated {
		if e := vm.deleteVM(); e != nil {
			glog.Errorf(constantes.ErrUnableToDeleteVM, vm.InstanceName, e)
		}
	} else {
		glog.Warningf(constantes.WarnFailedVMNotDeleted, vm.InstanceName, status)
	}

}
