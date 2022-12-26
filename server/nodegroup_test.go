package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	managednodeClientset "github.com/Fred78290/kubernetes-aws-autoscaler/pkg/generated/clientset/versioned"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type baseTest struct {
	t *testing.T
}

type nodegroupTest struct {
	baseTest
}

type autoScalerServerNodeGroupTest struct {
	AutoScalerServerNodeGroup
	t         *testing.T
	awsConfig *aws.Configuration
}

func (ng *autoScalerServerNodeGroupTest) createTestNode(name ...string) *AutoScalerServerNode {
	nodeName := testNodeName

	if len(name) > 0 {
		nodeName = name[0]
	}

	if machine, ok := ng.configuration.Machines[ng.InstanceType]; ok {
		runningInstance, state := findEc2Instance(ng.awsConfig, nodeName)

		node := &AutoScalerServerNode{
			NodeGroupID:     testGroupID,
			InstanceName:    nodeName,
			NodeName:        nodeName,
			InstanceType:    ng.InstanceType,
			DiskType:        machine.DiskType,
			DiskSize:        machine.DiskSize,
			IPAddress:       *runningInstance.AddressIP,
			State:           state,
			NodeType:        AutoScalerServerNodeAutoscaled,
			awsConfig:       ng.awsConfig,
			serverConfig:    ng.configuration,
			runningInstance: runningInstance,
		}

		ng.Nodes[nodeName] = node
		ng.RunningNodes[len(ng.RunningNodes)+1] = ServerNodeStateRunning

		return node
	}

	ng.t.Fatalf("Unable to find machine definition for type: %s", ng.InstanceType)

	return nil
}

func (m *nodegroupTest) launchVM() {
	ng, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.launchVM(m, ng.NodeLabels, ng.SystemLabels); err != nil {
			m.t.Errorf("AutoScalerNode.launchVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) startVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.startVM(m); err != nil {
			m.t.Errorf("AutoScalerNode.startVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) stopVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.stopVM(m); err != nil {
			m.t.Errorf("AutoScalerNode.stopVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := testNode.deleteVM(m); err != nil {
			m.t.Errorf("AutoScalerNode.deleteVM() error = %v", err)
		}
	}
}

func (m *nodegroupTest) statusVM() {
	_, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if got, err := testNode.statusVM(); err != nil {
			m.t.Errorf("AutoScalerNode.statusVM() error = %v", err)
		} else if got != AutoScalerServerNodeStateRunning {
			m.t.Errorf("AutoScalerNode.statusVM() = %v, want %v", got, AutoScalerServerNodeStateRunning)
		}
	}
}

func (m *nodegroupTest) addNode() {
	ng, err := m.newTestNodeGroup()

	if assert.NoError(m.t, err) {
		if _, err := ng.addNodes(m, 1); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.addNode() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteNode() {
	ng, testNode, err := m.newTestNode(launchVMName)

	if assert.NoError(m.t, err) {
		if err := ng.deleteNodeByName(m, testNode.NodeName); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.deleteNode() error = %v", err)
		}
	}
}

func (m *nodegroupTest) deleteNodeGroup() {
	ng, err := m.newTestNodeGroup()

	if assert.NoError(m.t, err) {
		if err := ng.deleteNodeGroup(m); err != nil {
			m.t.Errorf("AutoScalerServerNodeGroup.deleteNodeGroup() error = %v", err)
		}
	}
}

func (m *baseTest) fixAnnotation(node *apiv1.Node) {
}

func (m *baseTest) KubeClient() (kubernetes.Interface, error) {
	return nil, nil
}

func (m *baseTest) NodeManagerClient() (managednodeClientset.Interface, error) {
	return nil, nil
}

func (m *baseTest) ApiExtentionClient() (apiextension.Interface, error) {
	return nil, nil
}

func (m *baseTest) PodList(nodeName string, podFilter types.PodFilterFunc) ([]apiv1.Pod, error) {
	return nil, nil
}

func (m *baseTest) NodeList() (*apiv1.NodeList, error) {
	node := apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
			UID:  testCRDUID,
			Annotations: map[string]string{
				constantes.AnnotationNodeGroupName:        testGroupID,
				constantes.AnnotationNodeIndex:            "0",
				constantes.AnnotationInstanceID:           testInstanceID,
				constantes.AnnotationNodeAutoProvisionned: "true",
				constantes.AnnotationScaleDownDisabled:    "false",
				constantes.AnnotationNodeManaged:          "false",
			},
		},
	}

	m.fixAnnotation(&node)

	return &apiv1.NodeList{
		Items: []apiv1.Node{
			node,
		},
	}, nil
}

func (m *baseTest) UncordonNode(nodeName string) error {
	return nil
}

func (m *baseTest) CordonNode(nodeName string) error {
	return nil
}

func (m *baseTest) SetProviderID(nodeName, providerID string) error {
	return nil
}

func (m *baseTest) MarkDrainNode(nodeName string) error {
	return nil
}

func (m *baseTest) DrainNode(nodeName string, ignoreDaemonSet, deleteLocalData bool) error {
	return nil
}

func (m *baseTest) GetNode(nodeName string) (*apiv1.Node, error) {
	node := &apiv1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
			UID:  testCRDUID,
			Annotations: map[string]string{
				constantes.AnnotationNodeGroupName:        testGroupID,
				constantes.AnnotationNodeIndex:            "0",
				constantes.AnnotationInstanceID:           testInstanceID,
				constantes.AnnotationNodeAutoProvisionned: "true",
				constantes.AnnotationScaleDownDisabled:    "false",
				constantes.AnnotationNodeManaged:          "false",
			},
		},
	}

	m.fixAnnotation(node)

	return node, nil
}

func (m *baseTest) DeleteNode(nodeName string) error {
	return nil
}

func (m *baseTest) AnnoteNode(nodeName string, annotations map[string]string) error {
	return nil
}

func (m *baseTest) LabelNode(nodeName string, labels map[string]string) error {
	return nil
}

func (m *baseTest) TaintNode(nodeName string, taints ...apiv1.Taint) error {
	return nil
}

func (m *baseTest) WaitNodeToBeReady(nodeName string, timeToWaitInSeconds int) error {
	return nil
}

func (m *baseTest) newTestNode(name ...string) (*autoScalerServerNodeGroupTest, *AutoScalerServerNode, error) {
	if ng, err := m.newTestNodeGroup(); err == nil {
		vm := ng.createTestNode(name...)

		return ng, vm, err
	} else {
		return nil, nil, err
	}
}

func (m *baseTest) newTestNodeGroup() (*autoScalerServerNodeGroupTest, error) {
	config, err := m.newTestConfig()

	if err == nil {
		ng := &autoScalerServerNodeGroupTest{
			t:         m.t,
			awsConfig: config.GetAwsConfiguration(testGroupID),
			AutoScalerServerNodeGroup: AutoScalerServerNodeGroup{
				AutoProvision:              true,
				ServiceIdentifier:          config.ServiceIdentifier,
				NodeGroupIdentifier:        testGroupID,
				ProvisionnedNodeNamePrefix: config.ProvisionnedNodeNamePrefix,
				ManagedNodeNamePrefix:      config.ManagedNodeNamePrefix,
				ControlPlaneNamePrefix:     config.ControlPlaneNamePrefix,
				InstanceType:               config.DefaultMachineType,
				Status:                     NodegroupCreated,
				MinNodeSize:                config.MinNode,
				MaxNodeSize:                config.MaxNode,
				SystemLabels:               types.KubernetesLabel{},
				Nodes:                      make(map[string]*AutoScalerServerNode),
				RunningNodes:               make(map[int]ServerNodeState),
				pendingNodes:               make(map[string]*AutoScalerServerNode),
				configuration:              config,
				NodeLabels:                 config.NodeLabels,
			},
		}

		return ng, err
	}

	return nil, err
}

func (m *baseTest) getConfFile() string {
	if config := os.Getenv("TEST_CONFIG"); config != "" {
		return config
	}

	return "../test/local_config.json"
}

func (m *baseTest) newTestConfig() (*types.AutoScalerServerConfig, error) {
	var config types.AutoScalerServerConfig

	if configStr, err := os.ReadFile(m.getConfFile()); err != nil {
		return nil, err
	} else {
		err = json.Unmarshal(configStr, &config)

		if err != nil {
			return nil, err
		}

		config.SSH.TestMode = true

		return &config, nil
	}
}

func (m *baseTest) ssh() {
	config, err := m.newTestConfig()

	if assert.NoError(m.t, err) {
		if _, err = utils.Sudo(config.SSH, "127.0.0.1", 1, "ls"); err != nil {
			m.t.Errorf("SSH error = %v", err)
		}
	}
}

func findEc2Instance(awsConfig *aws.Configuration, nodeName string) (*aws.Ec2Instance, AutoScalerServerNodeState) {
	var state AutoScalerServerNodeState = AutoScalerServerNodeStateNotCreated
	var runningInstance *aws.Ec2Instance
	var err error

	if nodeName != testNodeName {
		if runningInstance, err = aws.GetEc2Instance(awsConfig, nodeName); err == nil {
			if status, err := runningInstance.Status(); err == nil {
				if status.Powered {
					state = AutoScalerServerNodeStateRunning
				}
			}
		} else {
			runningInstance = nil
		}
	}

	if runningInstance == nil {
		runningInstance, _ = aws.NewEc2Instance(awsConfig, nodeName)

		runningInstance.InstanceID = awssdk.String(testInstanceID)
		runningInstance.Region = awssdk.String(testRegion)
		runningInstance.Zone = awssdk.String(testZone)
		runningInstance.AddressIP = awssdk.String("127.0.0.1")
	}

	return runningInstance, state
}

func findInstanceID(awsConfig *aws.Configuration, nodeName string) string {
	instance, _ := findEc2Instance(awsConfig, nodeName)

	return *instance.InstanceID
}

func createTestNodegroup(t *testing.T) *nodegroupTest {
	return &nodegroupTest{
		baseTest: baseTest{
			t: t,
		},
	}
}

func Test_SSH(t *testing.T) {
	createTestNodegroup(t).ssh()
}

func TestNodeGroup_launchVM(t *testing.T) {
	createTestNodegroup(t).launchVM()
}

func TestNodeGroup_startVM(t *testing.T) {
	createTestNodegroup(t).startVM()
}

func TestNodeGroup_stopVM(t *testing.T) {
	createTestNodegroup(t).stopVM()
}

func TestNodeGroup_deleteVM(t *testing.T) {
	createTestNodegroup(t).deleteVM()
}

func TestNodeGroup_statusVM(t *testing.T) {
	createTestNodegroup(t).statusVM()
}

func TestNodeGroupGroup_addNode(t *testing.T) {
	createTestNodegroup(t).addNode()
}

func TestNodeGroupGroup_deleteNode(t *testing.T) {
	createTestNodegroup(t).deleteNode()
}

func TestNodeGroupGroup_deleteNodeGroup(t *testing.T) {
	createTestNodegroup(t).deleteNodeGroup()
}
