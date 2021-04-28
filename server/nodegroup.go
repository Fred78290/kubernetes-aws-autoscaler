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

// AutoScalerServerNodeGroup Group all AutoScaler VM created inside a NodeGroup
// Each node have name like <node group name>-vm-<vm index>
type AutoScalerServerNodeGroup struct {
	sync.Mutex
	NodeGroupIdentifier  string                           `json:"identifier"`
	ServiceIdentifier    string                           `json:"service"`
	InstanceType         string                           `json:"instance-type"`
	DiskSize             int                              `json:"diskSize"`
	Status               NodeGroupState                   `json:"status"`
	MinNodeSize          int                              `json:"minSize"`
	MaxNodeSize          int                              `json:"maxSize"`
	Nodes                map[string]*AutoScalerServerNode `json:"nodes"`
	NodeLabels           KubernetesLabel                  `json:"nodeLabels"`
	SystemLabels         KubernetesLabel                  `json:"systemLabels"`
	AutoProvision        bool                             `json:"auto-provision"`
	LastCreatedNodeIndex int                              `json:"node-index"`
	pendingNodes         map[string]*AutoScalerServerNode
	pendingNodesWG       sync.WaitGroup
	configuration        *types.AutoScalerServerConfig
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

	for nodeIndex := startIndex; nodeIndex >= endIndex; nodeIndex-- {
		nodeName := g.nodeName(nodeIndex)

		if node := g.Nodes[nodeName]; node != nil {
			tempNodes = append(tempNodes, node)

			if err = node.deleteVM(c); err != nil {
				glog.Errorf(constantes.ErrUnableToDeleteVM, node.InstanceName, err)
				break
			}
		}
	}

	for _, node := range tempNodes {
		delete(g.Nodes, node.InstanceName)
	}

	return err
}

func (g *AutoScalerServerNodeGroup) addNodes(c types.ClientGenerator, delta int) error {
	var err error

	glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s", g.NodeGroupIdentifier)

	tempNodes := make([]*AutoScalerServerNode, 0, delta)

	g.pendingNodesWG.Add(delta)

	for nodeIndex := 0; nodeIndex < delta; nodeIndex++ {
		if g.Status != NodegroupCreated {
			glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
			break
		}

		g.LastCreatedNodeIndex++

		nodeName := g.nodeName(g.LastCreatedNodeIndex)

		if awsConfig := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier); awsConfig != nil {

			node := &AutoScalerServerNode{
				ProviderID:       g.providerIDForNode(nodeName),
				NodeGroupID:      g.NodeGroupIdentifier,
				InstanceName:     nodeName,
				NodeName:         nodeName,
				NodeIndex:        g.LastCreatedNodeIndex,
				InstanceType:     g.InstanceType,
				Disk:             g.DiskSize,
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

	numberOfPendingNodes := len(tempNodes)

	if g.Status != NodegroupCreated {
		glog.Debugf("AutoScalerServerNodeGroup::addNodes, nodeGroupID:%s -> g.status != nodegroupCreated", g.NodeGroupIdentifier)
	} else if numberOfPendingNodes > 1 {

		// WaitGroup
		wg := sync.WaitGroup{}

		// Number of nodes to launc
		wg.Add(numberOfPendingNodes)

		type ReturnValue struct {
			node *AutoScalerServerNode
			err  error
		}

		// NodeGroup stopped error
		ngNotRunningErr := fmt.Errorf("nodegroup %s not running", g.NodeGroupIdentifier)

		// Collect returned error
		returns := make([]ReturnValue, numberOfPendingNodes)

		// Launch wrapper
		nodeLauncher := func(index int, node *AutoScalerServerNode) {
			returnValue := ReturnValue{
				node: node,
			}

			returns[index] = returnValue

			// Check if NG still running
			if g.Status == NodegroupCreated {
				go func() {
					returnValue.err = node.launchVM(c, g.NodeLabels, g.SystemLabels)

					// Remove from pending
					delete(g.pendingNodes, node.InstanceName)

					if returnValue.err != nil {
						node.cleanOnLaunchError(c, returnValue.err)
					} else {
						// Add node to running nodes
						g.Nodes[node.InstanceName] = node
					}

					// Pending node effective removed
					g.pendingNodesWG.Done()

					// Notify it's done
					wg.Done()
				}()
			} else {
				returnValue.err = ngNotRunningErr
			}
		}

		// Launch each node in background
		for nodeIndex, node := range tempNodes {
			nodeLauncher(nodeIndex, node)
		}

		// Wait all launched ended
		wg.Wait()

		// Analyse returns the first occured error
		for _, result := range returns {

			if result.err == ngNotRunningErr {
				glog.Debug("Ignore ng not running error")
			} else if result.err != nil {
				err = result.err
				break
			}
		}
	} else {
		node := tempNodes[0]

		delete(g.pendingNodes, node.InstanceName)

		if err = node.launchVM(c, g.NodeLabels, g.SystemLabels); err != nil {
			node.cleanOnLaunchError(c, err)
		} else {
			g.Nodes[node.InstanceName] = node
		}

		g.pendingNodesWG.Done()
	}

	return err
}

func (g *AutoScalerServerNodeGroup) autoDiscoveryNodes(client types.ClientGenerator, scaleDownDisabled bool) error {
	var lastNodeIndex = 0
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
	g.LastCreatedNodeIndex = 0

	awsConfig := g.configuration.GetAwsConfiguration(g.NodeGroupIdentifier)

	for _, nodeInfo := range nodeInfos.Items {
		var providerID = utils.GetNodeProviderID(g.ServiceIdentifier, &nodeInfo)
		var instanceName string

		if len(providerID) > 0 {
			out, _ = utils.NodeGroupIDFromProviderID(g.ServiceIdentifier, providerID)

			if out == g.NodeGroupIdentifier {
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

					if len(nodeInfo.Annotations[constantes.AnnotationNodeIndex]) != 0 {
						lastNodeIndex, _ = strconv.Atoi(nodeInfo.Annotations[constantes.AnnotationNodeIndex])
					}

					g.LastCreatedNodeIndex = utils.MaxInt(g.LastCreatedNodeIndex, lastNodeIndex)

					// Node name and instance name could be differ when using AWS cloud provider
					if ec2Instance, err = aws.GetEc2Instance(awsConfig, instanceName); err == nil {

						if node == nil {
							glog.Infof("Add node:%s with IP:%s to nodegroup:%s", instanceName, runningIP, g.NodeGroupIdentifier)

							node = &AutoScalerServerNode{
								ProviderID:       providerID,
								NodeGroupID:      g.NodeGroupIdentifier,
								InstanceName:     instanceName,
								NodeName:         nodeInfo.Name,
								NodeIndex:        lastNodeIndex,
								State:            AutoScalerServerNodeStateRunning,
								AutoProvisionned: nodeInfo.Annotations[constantes.AnnotationNodeAutoProvisionned] == "true",
								AwsConfig:        awsConfig,
								RunningInstance:  ec2Instance,
								Addresses: []string{
									runningIP,
								},
								serverConfig: g.configuration,
							}

							err = client.AnnoteNode(nodeInfo.Name, map[string]string{
								constantes.AnnotationScaleDownDisabled:    strconv.FormatBool(scaleDownDisabled && !node.AutoProvisionned),
								constantes.AnnotationInstanceName:         instanceName,
								constantes.AnnotationInstanceID:           *ec2Instance.InstanceID,
								constantes.AnnotationNodeAutoProvisionned: strconv.FormatBool(node.AutoProvisionned),
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

					lastNodeIndex++

					if node != nil {
						g.Nodes[instanceName] = node

						if _, err = node.statusVM(); err != nil {
							glog.Warnf("status return %v", err)
						}
					}
				}
			}
		}
	}

	return nil
}

func (g *AutoScalerServerNodeGroup) deleteNodeByName(c types.ClientGenerator, nodeName string) error {
	glog.Debugf("AutoScalerServerNodeGroup::deleteNodeByName, nodeGroupID:%s, nodeName:%s", g.NodeGroupIdentifier, nodeName)

	var err error

	if node := g.Nodes[nodeName]; node != nil {

		if err = node.deleteVM(c); err != nil {
			glog.Errorf(constantes.ErrUnableToDeleteVM, node.InstanceName, err)
		}

		delete(g.Nodes, nodeName)

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

func (g *AutoScalerServerNodeGroup) nodeName(vmIndex int) string {
	return fmt.Sprintf("%s-vm-%02d", g.NodeGroupIdentifier, vmIndex)
}

func (g *AutoScalerServerNodeGroup) providerID() string {
	return fmt.Sprintf("%s://%s/object?type=group", g.ServiceIdentifier, g.NodeGroupIdentifier)
}

func (g *AutoScalerServerNodeGroup) providerIDForNode(nodeName string) string {
	return fmt.Sprintf("%s://%s/object?type=node&name=%s", g.ServiceIdentifier, g.NodeGroupIdentifier, nodeName)
}
