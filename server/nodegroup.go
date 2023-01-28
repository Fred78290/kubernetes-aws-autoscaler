package server

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	v1alpha1 "github.com/Fred78290/kubernetes-aws-autoscaler/pkg/apis/nodemanager/v1alpha1"

	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uid "k8s.io/apimachinery/pkg/types"
)

// NodeGroupState describe the nodegroup status
type NodeGroupState int32

const (
	// NodegroupNotCreated not created state
	NodegroupNotCreated = iota

	// NodegroupCreated create state
	NodegroupCreated

	// NodegroupDeleting deleting status
	NodegroupDeleting

	// NodegroupDeleted deleted status
	NodegroupDeleted
)

type ServerNodeState int

const (
	ServerNodeStateNotRunning = iota
	ServerNodeStateDeleted
	ServerNodeStateCreating
	ServerNodeStateRunning
)

// AutoScalerServerNodeGroup Group all AutoScaler VM created inside a NodeGroup
// Each node have name like <node group name>-vm-<vm index>
type AutoScalerServerNodeGroup struct {
	sync.Mutex
	NodeGroupIdentifier        string                           `json:"identifier"`
	ServiceIdentifier          string                           `json:"service"`
	ProvisionnedNodeNamePrefix string                           `default:"autoscaled" json:"node-name-prefix"`
	ManagedNodeNamePrefix      string                           `default:"worker" json:"managed-name-prefix"`
	ControlPlaneNamePrefix     string                           `default:"master" json:"controlplane-name-prefix"`
	InstanceType               string                           `json:"instance-type"`
	DiskType                   string                           `default:"gp2" json:"diskType"`
	DiskSize                   int                              `json:"diskSize"`
	Status                     NodeGroupState                   `json:"status"`
	MinNodeSize                int                              `json:"minSize"`
	MaxNodeSize                int                              `json:"maxSize"`
	Nodes                      map[string]*AutoScalerServerNode `json:"nodes"`
	NodeLabels                 types.KubernetesLabel            `json:"nodeLabels"`
	SystemLabels               types.KubernetesLabel            `json:"systemLabels"`
	AutoProvision              bool                             `json:"auto-provision"`
	LastCreatedNodeIndex       int                              `json:"node-index"`
	RunningNodes               map[int]ServerNodeState          `json:"running-nodes-state"`
	pendingNodes               map[string]*AutoScalerServerNode
	pendingNodesWG             sync.WaitGroup
	numOfControlPlanes         int
	numOfExternalNodes         int
	numOfProvisionnedNodes     int
	numOfManagedNodes          int
	configuration              *types.AutoScalerServerConfig
}

func CreateLabelOrAnnotation(values []string) types.KubernetesLabel {
	result := types.KubernetesLabel{}

	for _, value := range values {
		if len(value) > 0 {
			parts := strings.Split(value, "=")

			if len(parts) == 1 {
				result[parts[0]] = ""
			} else {
				result[parts[0]] = parts[1]
			}
		}
	}

	return result
}

func (g *AutoScalerServerNodeGroup) findNextNodeIndex(managed bool) int {

	for index := 1; index <= g.MaxNodeSize; index++ {
		if run, found := g.RunningNodes[index]; !found || run < ServerNodeStateCreating {
			return index
		}
	}

	g.LastCreatedNodeIndex++

	return g.LastCreatedNodeIndex
}

func (g *AutoScalerServerNodeGroup) cleanup(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s", g.NodeGroupIdentifier)

	var lastError error

	g.Status = NodegroupDeleting

	g.pendingNodesWG.Wait()

	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s, iterate node to delete", g.NodeGroupIdentifier)

	for _, node := range g.AllNodes() {
		if node.NodeType == AutoScalerServerNodeAutoscaled {
			if lastError = node.deleteVM(c); lastError != nil {
				glog.Errorf(constantes.ErrNodeGroupCleanupFailOnVM, g.NodeGroupIdentifier, node.InstanceName, lastError)

				// Not fatal on cleanup
				if lastError.Error() == fmt.Sprintf(constantes.ErrVMNotFound, node.InstanceName) {
					lastError = nil
				}
			}
		}
	}

	g.RunningNodes = make(map[int]ServerNodeState)
	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.pendingNodes = make(map[string]*AutoScalerServerNode)
	g.numOfControlPlanes = 0
	g.numOfExternalNodes = 0
	g.numOfManagedNodes = 0
	g.numOfProvisionnedNodes = 0
	g.Status = NodegroupDeleted

	return lastError
}

func (g *AutoScalerServerNodeGroup) targetSize() int {
	glog.Debugf("AutoScalerServerNodeGroup::targetSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	return len(g.pendingNodes) + len(g.Nodes)
}

func (g *AutoScalerServerNodeGroup) AllNodes() []*AutoScalerServerNode {
	return append(utils.Values(g.Nodes), utils.Values(g.pendingNodes)...)
}

func (g *AutoScalerServerNodeGroup) setNodeGroupSize(c types.ClientGenerator, newSize int, prepareOnly bool) ([]*AutoScalerServerNode, error) {
	glog.Debugf("AutoScalerServerNodeGroup::setNodeGroupSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	g.Lock()
	defer g.Unlock()

	delta := newSize - g.targetSize()

	if delta < 0 {
		if prepareOnly {
			return g.prepareDeleteNodes(delta), nil
		} else {
			return g.deleteNodes(c, delta)
		}
	} else if delta > 0 {
		if prepareOnly {
			return g.prepareNodes(c, delta)
		} else {
			return g.addNodes(c, delta)
		}
	}

	return []*AutoScalerServerNode{}, nil
}

func (g *AutoScalerServerNodeGroup) refresh() {
	glog.Debugf("AutoScalerServerNodeGroup::refresh, nodeGroupID:%s", g.NodeGroupIdentifier)

	for _, node := range g.AllNodes() {
		if _, err := node.statusVM(); err != nil {
			glog.Infof("status VM return an error: %v", err)
		}
	}
}

func (g *AutoScalerServerNodeGroup) removeNamedNode(nodeName string) {
	delete(g.Nodes, nodeName)
	delete(g.pendingNodes, nodeName)
}

func (g *AutoScalerServerNodeGroup) findNamedNode(nodeName string) *AutoScalerServerNode {
	var node *AutoScalerServerNode = nil

	if node = g.Nodes[nodeName]; node == nil {
		node = g.pendingNodes[nodeName]
	}

	return node
}

func (g *AutoScalerServerNodeGroup) prepareDeleteNodes(delta int) []*AutoScalerServerNode {
	pendingNodes := utils.Values(g.pendingNodes)
	startIndex := len(pendingNodes) - 1
	tempNodes := make([]*AutoScalerServerNode, 0, -delta)

	for index := startIndex; index > 0; index-- {
		node := pendingNodes[index]

		// Don't delete not owned node
		if node.NodeType == AutoScalerServerNodeAutoscaled {
			delta++
			tempNodes = append(tempNodes, node)
		}

		if delta == 0 {
			break
		}
	}

	return tempNodes
}

func (g *AutoScalerServerNodeGroup) destroyNode(c types.ClientGenerator, node *AutoScalerServerNode) error {
	g.RunningNodes[node.NodeIndex] = ServerNodeStateDeleted
	g.removeNamedNode(node.InstanceName)

	return node.deleteVM(c)
}

func (g *AutoScalerServerNodeGroup) destroyNodes(c types.ClientGenerator, nodes []*AutoScalerServerNode) ([]*AutoScalerServerNode, error) {
	var err error
	deletedNodes := make([]*AutoScalerServerNode, 0, len(nodes))

	for _, node := range nodes {
		if err = g.destroyNode(c, node); err != nil {
			glog.Errorf(constantes.ErrUnableToDeleteVM, node.InstanceName, err)
		} else {
			deletedNodes = append(deletedNodes, node)
		}
	}

	return deletedNodes, err
}

// delta must be negative and will delete nodes in pending!!!!
func (g *AutoScalerServerNodeGroup) deleteNodes(c types.ClientGenerator, delta int) ([]*AutoScalerServerNode, error) {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	return g.destroyNodes(c, g.prepareDeleteNodes(-delta))
}

func (g *AutoScalerServerNodeGroup) addManagedNode(crd *v1alpha1.ManagedNode) (*AutoScalerServerNode, error) {
	controlPlane := crd.Spec.ControlPlane
	nodeName, nodeIndex := g.nodeName(g.findNextNodeIndex(true), controlPlane, true)

	if awsConfig := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier); awsConfig != nil {
		var desiredENI *aws.UserDefinedNetworkInterface
		var instanceType = crd.Spec.InstanceType

		g.RunningNodes[nodeIndex] = ServerNodeStateCreating

		resLimit := g.configuration.ManagedNodeResourceLimiter

		diskSize := utils.MaxInt(utils.MinInt(crd.Spec.DiskSize, resLimit.GetMaxValue(constantes.ResourceNameManagedNodeDisk, types.ManagedNodeMaxDiskSize)),
			resLimit.GetMinValue(constantes.ResourceNameManagedNodeDisk, types.ManagedNodeMinDiskSize))

		if len(instanceType) == 0 {
			instanceType = g.InstanceType
		}

		if crd.Spec.ENI != nil {
			eni := crd.Spec.ENI
			if len(eni.SubnetID)+len(eni.SecurityGroupID) > 0 {
				desiredENI = &aws.UserDefinedNetworkInterface{
					NetworkInterfaceID: eni.NetworkInterfaceID,
					SubnetID:           eni.SubnetID,
					SecurityGroupID:    eni.SecurityGroupID,
					PrivateAddress:     eni.PrivateAddress,
					PublicIP:           eni.PublicIP,
				}
			}
		}

		node := &AutoScalerServerNode{
			NodeGroupID:      g.NodeGroupIdentifier,
			NodeName:         nodeName,
			InstanceName:     nodeName,
			InstanceType:     instanceType,
			NodeIndex:        nodeIndex,
			DiskSize:         diskSize,
			DiskType:         g.DiskType,
			NodeType:         AutoScalerServerNodeManaged,
			ControlPlaneNode: controlPlane,
			AllowDeployment:  crd.Spec.AllowDeployment,
			ExtraLabels:      CreateLabelOrAnnotation(crd.Spec.Labels),
			ExtraAnnotations: CreateLabelOrAnnotation(crd.Spec.Annotations),
			CRDUID:           crd.GetUID(),
			awsConfig:        awsConfig,
			serverConfig:     g.configuration,
			desiredENI:       desiredENI,
		}

		annoteMaster := ""

		if g.configuration.UseK3S != nil && *g.configuration.UseK3S {
			annoteMaster = "true"
		}

		// Add system labels
		if controlPlane {
			node.ExtraLabels[constantes.NodeLabelMasterRole] = annoteMaster
			node.ExtraLabels[constantes.NodeLabelControlPlaneRole] = annoteMaster
			node.ExtraLabels["master"] = "true"
		} else {
			node.ExtraLabels[constantes.NodeLabelWorkerRole] = annoteMaster
			node.ExtraLabels["worker"] = "true"
		}

		g.pendingNodes[node.InstanceName] = node

		return node, nil
	} else {
		return nil, fmt.Errorf("aws configuration got error")
	}
}

func (g *AutoScalerServerNodeGroup) prepareNodes(c types.ClientGenerator, delta int) ([]*AutoScalerServerNode, error) {
	tempNodes := make([]*AutoScalerServerNode, 0, delta)
	annoteMaster := ""

	if g.configuration.UseK3S != nil && *g.configuration.UseK3S {
		annoteMaster = "true"
	}

	if g.Status != NodegroupCreated {
		glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
		return []*AutoScalerServerNode{}, fmt.Errorf(constantes.ErrNodeGroupNotFound, g.NodeGroupIdentifier)
	}

	for {
		nodeName, nodeIndex := g.nodeName(g.findNextNodeIndex(false), false, false)

		if awsConfig := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier); awsConfig != nil {

			g.RunningNodes[nodeIndex] = ServerNodeStateCreating

			extraAnnotations := types.KubernetesLabel{}
			extraLabels := types.KubernetesLabel{
				constantes.NodeLabelWorkerRole: annoteMaster,
				"worker":                       "true",
			}

			node := &AutoScalerServerNode{
				NodeGroupID:      g.NodeGroupIdentifier,
				InstanceName:     nodeName,
				NodeName:         nodeName,
				NodeIndex:        nodeIndex,
				InstanceType:     g.InstanceType,
				DiskType:         g.DiskType,
				DiskSize:         g.DiskSize,
				NodeType:         AutoScalerServerNodeAutoscaled,
				ExtraAnnotations: extraAnnotations,
				ExtraLabels:      extraLabels,
				ControlPlaneNode: false,
				AllowDeployment:  true,
				awsConfig:        awsConfig,
				serverConfig:     g.configuration,
			}

			tempNodes = append(tempNodes, node)

			g.pendingNodes[node.InstanceName] = node

			delta--

			if delta == 0 {
				break
			}
		} else {
			g.pendingNodes = make(map[string]*AutoScalerServerNode)
			return []*AutoScalerServerNode{}, fmt.Errorf("unable to find node group named %s", g.NodeGroupIdentifier)
		}
	}

	return tempNodes, nil
}

func (g *AutoScalerServerNodeGroup) addNodes(c types.ClientGenerator, delta int) ([]*AutoScalerServerNode, error) {
	glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	if tempNodes, err := g.prepareNodes(c, delta); err == nil {
		return g.createNodes(c, tempNodes)
	} else {
		return tempNodes, err
	}
}

// return the list of successfuly created nodes
func (g *AutoScalerServerNodeGroup) createNodes(c types.ClientGenerator, nodes []*AutoScalerServerNode) ([]*AutoScalerServerNode, error) {
	var mu sync.Mutex
	createdNodes := make([]*AutoScalerServerNode, 0, len(nodes))

	glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	numberOfNodesToCreate := len(nodes)

	createNode := func(node *AutoScalerServerNode) error {
		var err error

		defer g.pendingNodesWG.Done()

		if g.Status != NodegroupCreated {
			g.RunningNodes[node.NodeIndex] = ServerNodeStateNotRunning
			glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			return fmt.Errorf(constantes.ErrUnableToLaunchVMNodeGroupNotReady, node.InstanceName)
		}

		if err = node.launchVM(c, g.NodeLabels, g.SystemLabels); err != nil {
			glog.Errorf(constantes.ErrUnableToLaunchVM, node.InstanceName, err)

			node.cleanOnLaunchError(c, err)

			mu.Lock()
			defer mu.Unlock()

			g.RunningNodes[node.NodeIndex] = ServerNodeStateDeleted
		} else {
			mu.Lock()
			defer mu.Unlock()

			createdNodes = append(createdNodes, node)

			g.Nodes[node.InstanceName] = node
			g.RunningNodes[node.NodeIndex] = ServerNodeStateRunning

			if node.NodeType == AutoScalerServerNodeAutoscaled {
				g.numOfProvisionnedNodes++
			} else if node.NodeType == AutoScalerServerNodeManaged {
				g.numOfManagedNodes++

				if node.ControlPlaneNode {
					g.numOfControlPlanes++
				}
			}
		}

		delete(g.pendingNodes, node.InstanceName)

		return err
	}

	var result error = nil
	var successful int

	g.pendingNodesWG.Add(numberOfNodesToCreate)

	// Do sync if one node only
	if numberOfNodesToCreate == 1 {
		if err := createNode(nodes[0]); err == nil {
			successful++
		}
	} else {
		var maxCreatedNodePerCycle int

		if g.configuration.MaxCreatedNodePerCycle <= 0 {
			maxCreatedNodePerCycle = numberOfNodesToCreate
		} else {
			maxCreatedNodePerCycle = g.configuration.MaxCreatedNodePerCycle
		}

		glog.Debugf("Launch node group %s of %d VM by %d nodes per cycle", g.NodeGroupIdentifier, numberOfNodesToCreate, maxCreatedNodePerCycle)

		createNodeAsync := func(currentNode *AutoScalerServerNode, wg *sync.WaitGroup, err chan error) {
			e := createNode(currentNode)
			wg.Done()
			err <- e
		}

		totalLoop := numberOfNodesToCreate / maxCreatedNodePerCycle

		if numberOfNodesToCreate%maxCreatedNodePerCycle > 0 {
			totalLoop++
		}

		currentNodeIndex := 0

		for numberOfCycle := 0; numberOfCycle < totalLoop; numberOfCycle++ {
			// WaitGroup per segment
			numberOfNodeInCycle := utils.MinInt(maxCreatedNodePerCycle, numberOfNodesToCreate-currentNodeIndex)

			glog.Debugf("Launched cycle: %d, node per cycle is: %d", numberOfCycle, numberOfNodeInCycle)

			if numberOfNodeInCycle > 1 {
				ch := make([]chan error, numberOfNodeInCycle)
				wg := sync.WaitGroup{}

				for nodeInCycle := 0; nodeInCycle < numberOfNodeInCycle; nodeInCycle++ {
					node := nodes[currentNodeIndex]
					cherr := make(chan error)
					ch[nodeInCycle] = cherr

					currentNodeIndex++

					wg.Add(1)

					go createNodeAsync(node, &wg, cherr)
				}

				// Wait this segment to launch
				glog.Debugf("Wait cycle to finish: %d", numberOfCycle)

				wg.Wait()

				glog.Debugf("Launched cycle: %d, collect result", numberOfCycle)

				for _, chError := range ch {
					if chError != nil {
						var err = <-chError

						if err == nil {
							successful++
						}
					}
				}

				glog.Debugf("Finished cycle: %d", numberOfCycle)
			} else if numberOfNodeInCycle > 0 {
				node := nodes[currentNodeIndex]
				currentNodeIndex++
				if err := createNode(node); err == nil {
					successful++
				}
			} else {
				break
			}
		}
	}

	g.pendingNodesWG.Wait()

	if g.Status != NodegroupCreated {
		result = fmt.Errorf(constantes.ErrUnableToLaunchNodeGroupNotCreated, g.NodeGroupIdentifier)

		glog.Errorf("Launched node group %s of %d VM got an error because was destroyed", g.NodeGroupIdentifier, numberOfNodesToCreate)
	} else if successful == 0 {
		result = fmt.Errorf(constantes.ErrUnableToLaunchNodeGroup, g.NodeGroupIdentifier)
		glog.Infof("Launched node group %s of %d VM failed", g.NodeGroupIdentifier, numberOfNodesToCreate)
	} else {
		glog.Infof("Launched node group %s of %d/%d VM successful", g.NodeGroupIdentifier, successful, numberOfNodesToCreate)
	}

	return createdNodes, result
}

func (g *AutoScalerServerNodeGroup) nodeAllowDeployment(nodeInfo *apiv1.Node) bool {

	if nodeInfo.Spec.Taints != nil {
		for _, taint := range nodeInfo.Spec.Taints {
			if taint.Key == constantes.NodeLabelMasterRole || taint.Key == constantes.NodeLabelControlPlaneRole {
				if taint.Effect == apiv1.TaintEffectNoSchedule {
					return true
				}
			}
		}
	}

	return true
}

func (g *AutoScalerServerNodeGroup) hasInstance(nodeName string) (bool, error) {
	glog.Debugf("AutoScalerServerNodeGroup::hasInstance, nodeGroupID:%s, nodeName:%s", g.NodeGroupIdentifier, nodeName)

	if node := g.findNamedNode(nodeName); node != nil {
		if status, err := node.statusVM(); err != nil {
			return false, err
		} else {
			return status == AutoScalerServerNodeStateRunning, nil
		}
	}

	return false, fmt.Errorf(constantes.ErrNodeNotFoundInNodeGroup, nodeName, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) findManagedNodeDeleted(client types.ClientGenerator, formerNodes map[string]*AutoScalerServerNode) {

	for nodeName, formerNode := range formerNodes {
		if found := g.findNamedNode(nodeName); found == nil {
			if _, err := formerNode.statusVM(); err == nil {
				glog.Infof("Node '%s' is deleted, delete VM", nodeName)
				if err := formerNode.deleteVM(client); err != nil {
					glog.Errorf(constantes.ErrUnableToDeleteVM, nodeName, err)
				}
			}
		}
	}
}

func (g *AutoScalerServerNodeGroup) autoDiscoveryNodes(client types.ClientGenerator, includeExistingNode bool) (map[string]*AutoScalerServerNode, error) {
	var lastNodeIndex = 0
	var ec2Instance *aws.Ec2Instance
	var nodeInfos *apiv1.NodeList
	var err error

	if nodeInfos, err = client.NodeList(); err != nil {
		return nil, err
	}

	formerNodes := g.Nodes

	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.pendingNodes = make(map[string]*AutoScalerServerNode)
	g.RunningNodes = make(map[int]ServerNodeState)
	g.LastCreatedNodeIndex = 0
	g.numOfExternalNodes = 0
	g.numOfManagedNodes = 0
	g.numOfProvisionnedNodes = 0
	g.numOfControlPlanes = 0

	awsConfig := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier)

	for _, nodeInfo := range nodeInfos.Items {
		if nodegroupName, found := nodeInfo.Annotations[constantes.AnnotationNodeGroupName]; found {
			var instanceName string
			var nodeType AutoScalerServerNodeType
			var UID uid.UID

			autoProvisionned, _ := strconv.ParseBool(nodeInfo.Annotations[constantes.AnnotationNodeAutoProvisionned])
			managedNode, _ := strconv.ParseBool(nodeInfo.Annotations[constantes.AnnotationNodeManaged])
			controlPlane := false

			if _, found := nodeInfo.Labels[constantes.NodeLabelControlPlaneRole]; found {
				controlPlane = true
			} else if _, found := nodeInfo.Labels[constantes.NodeLabelMasterRole]; found {
				controlPlane = true
			}

			if autoProvisionned {
				nodeType = AutoScalerServerNodeAutoscaled
			} else if managedNode {
				nodeType = AutoScalerServerNodeManaged
			} else {
				nodeType = AutoScalerServerNodeExternal
			}

			// Ignore nodes not handled by autoscaler if option includeExistingNode == false
			if nodegroupName == g.NodeGroupIdentifier && (autoProvisionned || includeExistingNode) {
				glog.Infof("Discover node:%s matching nodegroup:%s", nodeInfo.Name, g.NodeGroupIdentifier)

				if instanceName, found = nodeInfo.Annotations[constantes.AnnotationInstanceName]; found {
					node := formerNodes[instanceName]
					runningIP := ""

					for _, address := range nodeInfo.Status.Addresses {
						if address.Type == apiv1.NodeInternalIP {
							runningIP = address.Address
							break
						}
					}

					glog.Infof("Found node:%s with IP:%s declared nodegroup:%s", instanceName, runningIP, g.NodeGroupIdentifier)

					if len(nodeInfo.Annotations[constantes.AnnotationNodeIndex]) != 0 {
						lastNodeIndex, _ = strconv.Atoi(nodeInfo.Annotations[constantes.AnnotationNodeIndex])
					}

					g.LastCreatedNodeIndex = utils.MaxInt(g.LastCreatedNodeIndex, lastNodeIndex)

					if ownerRef := metav1.GetControllerOf(&nodeInfo); ownerRef != nil {
						if ownerRef.Kind == v1alpha1.SchemeGroupVersionKind.Kind {
							UID = ownerRef.UID
						}
					}

					// Node name and instance name could be differ when using AWS cloud provider
					if ec2Instance, err = aws.GetEc2Instance(awsConfig, instanceName); err == nil {
						if node == nil {
							glog.Infof("Add node:%s with IP:%s to nodegroup:%s", instanceName, runningIP, g.NodeGroupIdentifier)

							node = &AutoScalerServerNode{
								NodeGroupID:      g.NodeGroupIdentifier,
								InstanceName:     instanceName,
								NodeName:         nodeInfo.Name,
								NodeIndex:        lastNodeIndex,
								State:            AutoScalerServerNodeStateRunning,
								NodeType:         nodeType,
								CRDUID:           UID,
								ControlPlaneNode: controlPlane,
								AllowDeployment:  g.nodeAllowDeployment(&nodeInfo),
								CPU:              int(nodeInfo.Status.Capacity.Cpu().Value()),
								Memory:           int(nodeInfo.Status.Capacity.Memory().Value() / (1024 * 1024)),
								DiskSize:         int(nodeInfo.Status.Capacity.Storage().Value() / (1024 * 1024)),
								awsConfig:        awsConfig,
								runningInstance:  ec2Instance,
								IPAddress:        runningIP,
								serverConfig:     g.configuration,
							}

							err = client.AnnoteNode(nodeInfo.Name, map[string]string{
								constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(nodeType != AutoScalerServerNodeAutoscaled),
								constantes.AnnotationNodeGroupName:        g.NodeGroupIdentifier,
								constantes.AnnotationInstanceName:         instanceName,
								constantes.AnnotationInstanceID:           *ec2Instance.InstanceID,
								constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(autoProvisionned),
								constantes.AnnotationNodeManaged:          strconv.FormatBool(managedNode),
								constantes.AnnotationNodeIndex:            strconv.Itoa(node.NodeIndex),
							})

							if err != nil {
								glog.Errorf(constantes.ErrAnnoteNodeReturnError, nodeInfo.Name, err)
							}

							err = client.LabelNode(nodeInfo.Name, map[string]string{
								constantes.AnnotationNodeGroupName: g.NodeGroupIdentifier,
							})

							if err != nil {
								glog.Errorf(constantes.ErrLabelNodeReturnError, nodeInfo.Name, err)
							}
						} else {
							node.runningInstance = ec2Instance
							node.serverConfig = g.configuration
							node.awsConfig = awsConfig

							glog.Infof("Attach existing node:%s with IP:%s to nodegroup:%s", instanceName, runningIP, g.NodeGroupIdentifier)
						}

						g.Nodes[nodeInfo.Name] = node
						g.RunningNodes[lastNodeIndex] = ServerNodeStateRunning

						if controlPlane {
							if managedNode {
								g.numOfManagedNodes++
							} else {
								g.numOfExternalNodes++
							}

							g.numOfControlPlanes++

						} else if autoProvisionned {
							g.numOfProvisionnedNodes++
						} else if managedNode {
							g.numOfManagedNodes++
						} else {
							g.numOfExternalNodes++
						}

						lastNodeIndex++

						_, _ = node.statusVM()
					} else {
						glog.Errorf("Can not add node:%s with IP:%s to nodegroup:%s, reason: %v", instanceName, runningIP, g.NodeGroupIdentifier, err)
					}
				}
			} else {
				glog.Infof("Ignore kubernetes node %s not handled by me", nodeInfo.Name)
			}
		}
	}

	return formerNodes, nil
}

func (g *AutoScalerServerNodeGroup) deleteNode(c types.ClientGenerator, node *AutoScalerServerNode) error {
	var err error

	if err = node.deleteVM(c); err != nil {
		glog.Errorf(constantes.ErrUnableToDeleteVM, node.InstanceName, err)
	}

	g.RunningNodes[node.NodeIndex] = ServerNodeStateDeleted
	g.removeNamedNode(node.InstanceName)

	if node.NodeType == AutoScalerServerNodeAutoscaled {
		g.numOfProvisionnedNodes--
	} else {
		g.numOfManagedNodes--
	}

	return err
}

func (g *AutoScalerServerNodeGroup) deleteNodeByName(c types.ClientGenerator, nodeName string) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeByName, nodeGroupID:%s, nodeName:%s", g.NodeGroupIdentifier, nodeName)

	if node := g.findNamedNode(nodeName); node != nil {

		return g.deleteNode(c, node)
	}

	return fmt.Errorf(constantes.ErrNodeNotFoundInNodeGroup, nodeName, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) setConfiguration(config *types.AutoScalerServerConfig) {
	glog.Debugf("AutoScalerServerNodeGroup::setConfiguration, nodeGroupID:%s", g.NodeGroupIdentifier)

	g.configuration = config

	for _, node := range g.AllNodes() {
		node.setServerConfiguration(config)
	}
}

func (g *AutoScalerServerNodeGroup) deleteNodeGroup(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeGroup, nodeGroupID:%s", g.NodeGroupIdentifier)

	return g.cleanup(c)
}

func (g *AutoScalerServerNodeGroup) getProvisionnedNodePrefix() string {
	if len(g.ProvisionnedNodeNamePrefix) == 0 {
		return "autoscaled"
	}

	return g.ProvisionnedNodeNamePrefix
}

func (g *AutoScalerServerNodeGroup) getControlPlanePrefix() string {
	if len(g.ControlPlaneNamePrefix) == 0 {
		return "master"
	}

	return g.ControlPlaneNamePrefix
}

func (g *AutoScalerServerNodeGroup) getManagedNodePrefix() string {
	if len(g.ManagedNodeNamePrefix) == 0 {
		return "worker"
	}

	return g.ManagedNodeNamePrefix
}

func (g *AutoScalerServerNodeGroup) nodeName(vmIndex int, controlplane, managed bool) (string, int) {
	var start int
	config := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier)

	if controlplane {
		start = 2
	} else {
		start = 1
	}

	for index := start; index <= g.MaxNodeSize; index++ {
		var nodeName string

		if controlplane {
			nodeName = fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getControlPlanePrefix(), index)
		} else if managed {
			nodeName = fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getManagedNodePrefix(), index)
		} else {
			nodeName = fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getProvisionnedNodePrefix(), index)
		}

		if found := g.findNamedNode(nodeName); found == nil {
			if !config.Exists(nodeName) {
				return nodeName, vmIndex
			} else {
				glog.Warnf(constantes.ErrVMAlreadyExists, nodeName)
				g.RunningNodes[vmIndex] = ServerNodeStateRunning
				vmIndex++
			}
		}
	}

	// Should never reach this code
	if controlplane {
		return fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getControlPlanePrefix(), vmIndex-g.numOfExternalNodes-g.numOfProvisionnedNodes-g.numOfManagedNodes+g.numOfControlPlanes+1), vmIndex
	} else if managed {
		return fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getManagedNodePrefix(), vmIndex-g.numOfExternalNodes-g.numOfProvisionnedNodes+1), vmIndex
	} else {
		return fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getProvisionnedNodePrefix(), vmIndex-g.numOfExternalNodes-g.numOfManagedNodes+1), vmIndex
	}
}

func (g *AutoScalerServerNodeGroup) findNodeByCRDUID(uid uid.UID) (*AutoScalerServerNode, error) {
	for _, node := range g.AllNodes() {
		if node.CRDUID == uid {
			return node, nil
		}
	}
	return nil, fmt.Errorf(constantes.ErrManagedNodeNotFound, uid)
}

func (g *AutoScalerServerNodeGroup) GetOptions(defaults *types.NodeGroupAutoscalingOptions) (*types.NodeGroupAutoscalingOptions, error) {
	if g.configuration.AutoScalingOptions != nil {
		return g.configuration.AutoScalingOptions, nil
	}

	return defaults, nil
}
