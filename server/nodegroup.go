package server

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"

	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
)

// NodeGroupState describe the nodegroup status
type NodeGroupState int32

const (
	// NodegroupNotCreated not created state
	NodegroupNotCreated NodeGroupState = 0

	// NodegroupCreated create state
	NodegroupCreated NodeGroupState = 1

	// NodegroupDeleting deleting status
	NodegroupDeleting NodeGroupState = 2

	// NodegroupDeleted deleted status
	NodegroupDeleted NodeGroupState = 3
)

// KubernetesLabel labels
type KubernetesLabel map[string]string

type ServerNodeState int

const (
	ServerNodeStateNotRunning ServerNodeState = 0
	ServerNodeStateDeleted    ServerNodeState = 1
	ServerNodeStateCreating   ServerNodeState = 2
	ServerNodeStateRunning    ServerNodeState = 3
)

// AutoScalerServerNodeGroup Group all AutoScaler VM created inside a NodeGroup
// Each node have name like <node group name>-vm-<vm index>
type AutoScalerServerNodeGroup struct {
	sync.Mutex
	NodeGroupIdentifier string                           `json:"identifier"`
	ServiceIdentifier   string                           `json:"service"`
	NodeNamePrefix      string                           `default:"autoscaled" json:"node-name-prefix"`
	InstanceType        string                           `json:"instance-type"`
	DiskType            string                           `default:"gp2" json:"diskType"`
	DiskSize            int                              `json:"diskSize"`
	Status              NodeGroupState                   `json:"status"`
	MinNodeSize         int                              `json:"minSize"`
	MaxNodeSize         int                              `json:"maxSize"`
	Nodes               map[string]*AutoScalerServerNode `json:"nodes"`
	NodeLabels          KubernetesLabel                  `json:"nodeLabels"`
	SystemLabels        KubernetesLabel                  `json:"systemLabels"`
	AutoProvision       bool                             `json:"auto-provision"`
	NotOwnedNodes       int                              `json:"not-owned-nodes"`
	LastNodeIndex       int                              `json:"last-node-index"`
	RunningNodes        map[int]ServerNodeState          `json:"running-nodes"`
	pendingNodes        map[string]*AutoScalerServerNode
	pendingNodesWG      sync.WaitGroup
	configuration       *types.AutoScalerServerConfig
}

func (g *AutoScalerServerNodeGroup) findNextNodeIndex() int {

	for index := 0; index <= g.LastNodeIndex; index++ {
		if run, found := g.RunningNodes[index]; !found || run < ServerNodeStateCreating {
			return index
		}
	}

	g.LastNodeIndex++

	return g.LastNodeIndex
}

func (g *AutoScalerServerNodeGroup) cleanup(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s", g.NodeGroupIdentifier)

	var lastError error

	g.Status = NodegroupDeleting

	g.pendingNodesWG.Wait()

	glog.Debugf("AutoScalerServerNodeGroup::cleanup, nodeGroupID:%s, iterate node to delete", g.NodeGroupIdentifier)

	for _, node := range g.Nodes {
		if lastError = node.deleteVM(c); lastError != nil {
			glog.Errorf(constantes.ErrNodeGroupCleanupFailOnVM, g.NodeGroupIdentifier, node.InstanceName, lastError)
		}
	}

	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.pendingNodes = make(map[string]*AutoScalerServerNode)
	g.Status = NodegroupDeleted

	return lastError
}

func (g *AutoScalerServerNodeGroup) targetSize() int {
	glog.Debugf("AutoScalerServerNodeGroup::targetSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	return len(g.pendingNodes) + len(g.Nodes)
}

func (g *AutoScalerServerNodeGroup) setNodeGroupSize(c types.ClientGenerator, newSize int) error {
	glog.Debugf("AutoScalerServerNodeGroup::setNodeGroupSize, nodeGroupID:%s", g.NodeGroupIdentifier)

	var err error

	g.Lock()

	delta := newSize - g.targetSize()

	if delta < 0 {
		err = g.deleteNodes(c, delta)
	} else if delta > 0 {
		err = g.addNodes(c, delta)
	}

	g.Unlock()

	return err
}

func (g *AutoScalerServerNodeGroup) refresh() {
	glog.Debugf("AutoScalerServerNodeGroup::refresh, nodeGroupID:%s", g.NodeGroupIdentifier)

	for _, node := range g.Nodes {
		if _, err := node.statusVM(); err != nil {
			glog.Infof("status VM return an error: %v", err)
		}
	}
}

// delta must be negative!!!!
func (g *AutoScalerServerNodeGroup) deleteNodes(c types.ClientGenerator, delta int) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	var err error

	startIndex := len(g.Nodes) - 1
	endIndex := startIndex + delta
	tempNodes := make([]*AutoScalerServerNode, 0, -delta)

	for index := startIndex; index >= endIndex; index-- {
		nodeName := g.nodeName(index)

		if node, found := g.Nodes[nodeName]; found {
			// Don't delete not owned node
			if node.AutoProvisionned {
				tempNodes = append(tempNodes, node)

				if err = node.deleteVM(c); err != nil {
					glog.Errorf(constantes.ErrUnableToDeleteVM, node.InstanceName, err)
					break
				}
			}
		}
	}

	for _, node := range tempNodes {
		g.RunningNodes[node.NodeIndex] = ServerNodeStateDeleted
		delete(g.Nodes, node.InstanceName)
	}

	return err
}

func (g *AutoScalerServerNodeGroup) addNodes(c types.ClientGenerator, delta int) error {
	glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	tempNodes := make([]*AutoScalerServerNode, 0, delta)

	for index := 0; index < delta; index++ {
		if g.Status != NodegroupCreated {
			glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			break
		}

		nodeIndex := g.findNextNodeIndex()
		nodeName := g.nodeName(nodeIndex)

		if awsConfig := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier); awsConfig != nil {

			g.RunningNodes[nodeIndex] = ServerNodeStateCreating

			node := &AutoScalerServerNode{
				ProviderID:       g.providerIDForNode(nodeName),
				NodeGroupID:      g.NodeGroupIdentifier,
				InstanceName:     nodeName,
				NodeName:         nodeName,
				NodeIndex:        nodeIndex,
				InstanceType:     g.InstanceType,
				DiskType:         g.DiskType,
				DiskSize:         g.DiskSize,
				AutoProvisionned: true,
				AwsConfig:        awsConfig,
				serverConfig:     g.configuration,
			}

			tempNodes = append(tempNodes, node)

			if g.pendingNodes == nil {
				g.pendingNodes = make(map[string]*AutoScalerServerNode)
			}

			g.pendingNodes[node.InstanceName] = node
		} else {
			return fmt.Errorf("unable to find node group named %s", g.NodeGroupIdentifier)
		}
	}

	createNode := func(node *AutoScalerServerNode) error {
		var err error

		defer g.pendingNodesWG.Done()

		if g.Status != NodegroupCreated {
			glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			return fmt.Errorf(constantes.ErrUnableToLaunchVMNodeGroupNotReady, node.InstanceName)
		}

		if err = node.launchVM(c, g.NodeLabels, g.SystemLabels); err != nil {
			glog.Errorf(constantes.ErrUnableToLaunchVM, node.NodeName, err)

			node.cleanOnLaunchError(c, err)

			g.RunningNodes[node.NodeIndex] = ServerNodeStateDeleted
		} else {
			g.Nodes[node.InstanceName] = node
			g.RunningNodes[node.NodeIndex] = ServerNodeStateRunning
		}

		delete(g.pendingNodes, node.InstanceName)

		return err
	}

	var result error = nil
	var successful int

	g.pendingNodesWG.Add(delta)

	// Do sync if one node only
	if len(tempNodes) == 1 {
		if err := createNode(tempNodes[0]); err == nil {
			successful++
		}
	} else {
		var maxCreatedNodePerCycle int

		if g.configuration.MaxCreatedNodePerCycle <= 0 {
			maxCreatedNodePerCycle = len(tempNodes)
		} else {
			maxCreatedNodePerCycle = g.configuration.MaxCreatedNodePerCycle
		}

		glog.Debugf("Launch node group %s of %d VM by %d nodes per cycle", g.NodeGroupIdentifier, delta, maxCreatedNodePerCycle)

		createNodeAsync := func(currentNode *AutoScalerServerNode, wg *sync.WaitGroup, err chan error) {
			e := createNode(currentNode)
			wg.Done()
			err <- e
		}

		totalLoop := len(tempNodes) / maxCreatedNodePerCycle

		if len(tempNodes)%maxCreatedNodePerCycle > 0 {
			totalLoop++
		}

		currentNodeIndex := 0

		for numberOfCycle := 0; numberOfCycle < totalLoop; numberOfCycle++ {
			// WaitGroup per segment
			numberOfNodeInCycle := utils.MinInt(maxCreatedNodePerCycle, len(tempNodes)-currentNodeIndex)

			glog.Debugf("Launched cycle: %d, node per cycle is: %d", numberOfCycle, numberOfNodeInCycle)

			if numberOfNodeInCycle > 1 {
				ch := make([]chan error, numberOfNodeInCycle)
				wg := sync.WaitGroup{}

				for nodeInCycle := 0; nodeInCycle < numberOfNodeInCycle; nodeInCycle++ {
					node := tempNodes[currentNodeIndex]
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
				node := tempNodes[currentNodeIndex]
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

		glog.Errorf("Launched node group %s of %d VM got an error because was destroyed", g.NodeGroupIdentifier, delta)
	} else if successful == 0 {
		result = fmt.Errorf(constantes.ErrUnableToLaunchNodeGroup, g.NodeGroupIdentifier)
		glog.Infof("Launched node group %s of %d VM failed", g.NodeGroupIdentifier, delta)
	} else {
		glog.Infof("Launched node group %s of %d/%d VM successful", g.NodeGroupIdentifier, successful, delta)
	}

	return result
}

func (g *AutoScalerServerNodeGroup) autoDiscoveryNodes(client types.ClientGenerator, includeExistingNode bool) error {
	var declaredNodeIndex = 0
	var ec2Instance *aws.Ec2Instance
	var nodeInfos *apiv1.NodeList
	var out string
	var err error

	if nodeInfos, err = client.NodeList(); err != nil {
		return err
	}

	formerNodes := g.Nodes

	g.Nodes = make(map[string]*AutoScalerServerNode)
	g.pendingNodes = make(map[string]*AutoScalerServerNode)
	g.RunningNodes = make(map[int]ServerNodeState)
	g.NotOwnedNodes = 0
	g.LastNodeIndex = 0

	awsConfig := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier)

	for _, nodeInfo := range nodeInfos.Items {
		var providerID = utils.GetNodeProviderID(g.ServiceIdentifier, &nodeInfo)
		var instanceName string

		if len(providerID) > 0 {
			out, _ = utils.NodeGroupIDFromProviderID(g.ServiceIdentifier, providerID)

			autoProvisionned, _ := strconv.ParseBool(nodeInfo.Annotations[constantes.AnnotationNodeAutoProvisionned])

			// Ignore nodes not handled by autoscaler if option includeExistingNode == false
			if out == g.NodeGroupIdentifier && (autoProvisionned || includeExistingNode) {
				glog.Infof("Discover node:%s matching nodegroup:%s", providerID, g.NodeGroupIdentifier)

				if instanceName, err = utils.NodeNameFromProviderID(g.ServiceIdentifier, providerID); err == nil {
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
						declaredNodeIndex, _ = strconv.Atoi(nodeInfo.Annotations[constantes.AnnotationNodeIndex])
					} else {
						declaredNodeIndex = g.LastNodeIndex + 1
					}

					g.LastNodeIndex = utils.MaxInt(g.LastNodeIndex, declaredNodeIndex)

					if !autoProvisionned {
						g.NotOwnedNodes++
					}

					// Node name and instance name could be differ when using AWS cloud provider
					if ec2Instance, err = aws.GetEc2Instance(awsConfig, instanceName); err == nil {
						if node == nil {
							glog.Infof("Add node:%s with IP:%s to nodegroup:%s", instanceName, runningIP, g.NodeGroupIdentifier)

							node = &AutoScalerServerNode{
								ProviderID:       providerID,
								NodeGroupID:      g.NodeGroupIdentifier,
								InstanceName:     instanceName,
								NodeName:         nodeInfo.Name,
								NodeIndex:        declaredNodeIndex,
								State:            AutoScalerServerNodeStateRunning,
								AutoProvisionned: autoProvisionned,
								AwsConfig:        awsConfig,
								RunningInstance:  ec2Instance,
								Addresses: []string{
									runningIP,
								},
								serverConfig: g.configuration,
							}

							err = client.AnnoteNode(nodeInfo.Name, map[string]string{
								constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(!autoProvisionned),
								constantes.NodeLabelGroupName:             g.NodeGroupIdentifier,
								constantes.AnnotationInstanceName:         instanceName,
								constantes.AnnotationInstanceID:           *ec2Instance.InstanceID,
								constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(autoProvisionned),
								constantes.AnnotationNodeIndex:            strconv.Itoa(node.NodeIndex),
							})

							if err != nil {
								glog.Errorf(constantes.ErrAnnoteNodeReturnError, nodeInfo.Name, err)
							}

							err = client.LabelNode(nodeInfo.Name, map[string]string{
								constantes.NodeLabelGroupName: g.NodeGroupIdentifier,
							})

							if err != nil {
								glog.Errorf(constantes.ErrLabelNodeReturnError, nodeInfo.Name, err)
							}
						} else {
							node.RunningInstance = ec2Instance
							node.serverConfig = g.configuration

							glog.Infof("Attach existing node:%s with IP:%s to nodegroup:%s", instanceName, runningIP, g.NodeGroupIdentifier)
						}

					} else {
						glog.Errorf("Can not add node:%s with IP:%s to nodegroup:%s, reason: %v", instanceName, runningIP, g.NodeGroupIdentifier, err)

						node = nil
					}

					if node != nil {
						g.Nodes[instanceName] = node
						g.RunningNodes[node.NodeIndex] = ServerNodeStateRunning

						if _, err = node.statusVM(); err != nil {
							glog.Warnf("status return %v", err)
						}
					}
				}
			} else {
				glog.Infof("Ignore kubernetes node %s not handled by me", nodeInfo.Name)
			}
		}
	}

	return nil
}

func (g *AutoScalerServerNodeGroup) deleteNodeByName(c types.ClientGenerator, nodeName string) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeByName, nodeGroupID:%s, nodeName:%s", g.NodeGroupIdentifier, nodeName)

	var err error

	if node, found := g.Nodes[nodeName]; found {

		if err = node.deleteVM(c); err != nil {
			glog.Errorf(constantes.ErrUnableToDeleteVM, node.InstanceName, err)
		}

		delete(g.Nodes, nodeName)

		g.RunningNodes[node.NodeIndex] = ServerNodeStateDeleted

		return err
	}

	return fmt.Errorf(constantes.ErrNodeNotFoundInNodeGroup, nodeName, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) setConfiguration(config *types.AutoScalerServerConfig) {
	glog.Debugf("AutoScalerServerNodeGroup::setConfiguration, nodeGroupID:%s", g.NodeGroupIdentifier)

	g.configuration = config

	for _, node := range g.Nodes {
		node.setServerConfiguration(config)
	}
}

func (g *AutoScalerServerNodeGroup) deleteNodeGroup(c types.ClientGenerator) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeGroup, nodeGroupID:%s", g.NodeGroupIdentifier)

	return g.cleanup(c)
}

func (g *AutoScalerServerNodeGroup) getNodePrefix() string {
	if len(g.NodeNamePrefix) == 0 {
		return "autoscaled"
	}

	return g.NodeNamePrefix
}

func (g *AutoScalerServerNodeGroup) nodeName(vmIndex int) string {
	return fmt.Sprintf("%s-%s-%02d", g.NodeGroupIdentifier, g.getNodePrefix(), vmIndex-g.NotOwnedNodes+1)
}

func (g *AutoScalerServerNodeGroup) providerID() string {
	return fmt.Sprintf("%s://%s/object?type=group", g.ServiceIdentifier, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) providerIDForNode(nodeName string) string {
	return fmt.Sprintf("%s://%s/object?type=node&name=%s", g.ServiceIdentifier, g.NodeGroupIdentifier, nodeName)
}
