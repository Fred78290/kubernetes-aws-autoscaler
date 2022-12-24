package server

import (
	"testing"

	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
)

func Test_Nodegroup(t *testing.T) {
	if utils.ShouldTestFeature("TestNodegroup") {
		test := nodegroupTest{t: t}

		if utils.ShouldTestFeature("TestNodeGroup_launchVM") {
			t.Run("TestNodeGroup_launchVM", func(t *testing.T) {
				test.launchVM()
			})
		}

		if utils.ShouldTestFeature("TestNodeGroup_stopVM") {
			t.Run("TestNodeGroup_stopVM", func(t *testing.T) {
				test.stopVM()
			})
		}

		if utils.ShouldTestFeature("TestNodeGroup_startVM") {
			t.Run("TestNodeGroup_startVM", func(t *testing.T) {
				test.startVM()
			})
		}

		if utils.ShouldTestFeature("TestNodeGroup_statusVM") {
			t.Run("TestNodeGroup_statusVM", func(t *testing.T) {
				test.statusVM()
			})
		}

		if utils.ShouldTestFeature("TestNodeGroup_deleteVM") {
			t.Run("TestNodeGroup_deleteVM", func(t *testing.T) {
				test.deleteVM()
			})
		}

		if utils.ShouldTestFeature("TestNodeGroupGroup_addNode") {
			t.Run("TestNodeGroupGroup_addNode", func(t *testing.T) {
				test.addNode()
			})
		}

		if utils.ShouldTestFeature("TestNodeGroupGroup_deleteNode") {
			t.Run("TestNodeGroupGroup_deleteNode", func(t *testing.T) {
				test.deleteNode()
			})
		}

		if utils.ShouldTestFeature("TestNodeGroupGroup_deleteNodeGroup") {
			t.Run("TestNodeGroupGroup_deleteNodeGroup", func(t *testing.T) {
				test.deleteNodeGroup()
			})
		}
	}
}

func Test_Server(t *testing.T) {
	if utils.ShouldTestFeature("TestServer") {
		test := serverTest{t: t}

		if utils.ShouldTestFeature("TestServer_NodeGroups") {
			t.Run("TestServer_NodeGroups", func(t *testing.T) {
				t.Log("Execute TestServer_NodeGroups")
				test.NodeGroups()
			})
		}

		if utils.ShouldTestFeature("TestServer_NodeGroupForNode") {
			t.Run("TestServer_NodeGroupForNode", func(t *testing.T) {
				t.Log("Execute TestServer_NodeGroupForNode")
				test.NodeGroupForNode()
			})
		}

		if utils.ShouldTestFeature("TestServer_Pricing") {
			t.Run("TestServer_Pricing", func(t *testing.T) {
				t.Log("Execute TestServer_Pricing")
				test.Pricing()
			})
		}

		if utils.ShouldTestFeature("TestServer_GetAvailableMachineTypes") {
			t.Run("TestServer_GetAvailableMachineTypes", func(t *testing.T) {
				t.Log("Execute TestServer_GetAvailableMachineTypes")
				test.GetAvailableMachineTypes()
			})
		}

		if utils.ShouldTestFeature("TestServer_NewNodeGroup") {
			t.Run("TestServer_NewNodeGroup", func(t *testing.T) {
				t.Log("Execute TestServer_NewNodeGroup")
				test.NewNodeGroup()
			})
		}

		if utils.ShouldTestFeature("TestServer_GetResourceLimiter") {
			t.Run("TestServer_GetResourceLimiter", func(t *testing.T) {
				t.Log("Execute TestServer_GetResourceLimiter")
				test.GetResourceLimiter()
			})
		}

		if utils.ShouldTestFeature("TestServer_Refresh") {
			t.Run("TestServer_Refresh", func(t *testing.T) {
				t.Log("Execute TestServer_Refresh")
				test.Refresh()
			})
		}

		if utils.ShouldTestFeature("TestServer_TargetSize") {
			t.Run("TestServer_TargetSize", func(t *testing.T) {
				t.Log("Execute TestServer_TargetSize")
				test.TargetSize()
			})
		}

		if utils.ShouldTestFeature("TestServer_IncreaseSize") {
			t.Run("TestServer_IncreaseSize", func(t *testing.T) {
				t.Log("Execute TestServer_IncreaseSize")
				test.IncreaseSize()
			})
		}

		if utils.ShouldTestFeature("TestServer_DecreaseTargetSize") {
			t.Run("TestServer_DecreaseTargetSize", func(t *testing.T) {
				t.Log("Execute TestServer_DecreaseTargetSize")
				test.DecreaseTargetSize()
			})
		}

		if utils.ShouldTestFeature("TestServer_DeleteNodes") {
			t.Run("TestServer_DeleteNodes", func(t *testing.T) {
				t.Log("Execute TestServer_DeleteNodes")
				test.DeleteNodes()
			})
		}

		if utils.ShouldTestFeature("TestServer_Id") {
			t.Run("TestServer_Id", func(t *testing.T) {
				t.Log("Execute TestServer_Id")
				test.Id()
			})
		}

		if utils.ShouldTestFeature("TestServer_Debug") {
			t.Run("TestServer_Debug", func(t *testing.T) {
				t.Log("Execute TestServer_Debug")
				test.Debug()
			})
		}

		if utils.ShouldTestFeature("TestServer_Nodes") {
			t.Run("TestServer_Nodes", func(t *testing.T) {
				t.Log("Execute TestServer_Nodes")
				test.Nodes()
			})
		}

		if utils.ShouldTestFeature("TestServer_TemplateNodeInfo") {
			t.Run("TestServer_TemplateNodeInfo", func(t *testing.T) {
				t.Log("Execute TestServer_TemplateNodeInfo")
				test.TemplateNodeInfo()
			})
		}

		if utils.ShouldTestFeature("TestServer_Exist") {
			t.Run("TestServer_Exist", func(t *testing.T) {
				t.Log("Execute TestServer_Exist")
				test.Exist()
			})
		}

		if utils.ShouldTestFeature("TestServer_Delete") {
			t.Run("TestServer_Delete", func(t *testing.T) {
				t.Log("Execute TestServer_Delete")
				test.Delete()
			})
		}

		if utils.ShouldTestFeature("TestServer_Create") {
			t.Run("TestServer_Create", func(t *testing.T) {
				t.Log("Execute TestServer_Create")
				test.Create()
			})
		}

		if utils.ShouldTestFeature("TestServer_Autoprovisioned") {
			t.Run("TestServer_Autoprovisioned", func(t *testing.T) {
				t.Log("Execute TestServer_Autoprovisioned")
				test.Autoprovisioned()
			})
		}

		if utils.ShouldTestFeature("TestServer_Belongs") {
			t.Run("TestServer_Belongs", func(t *testing.T) {
				t.Log("Execute TestServer_Belongs")
				test.Belongs()
			})
		}

		if utils.ShouldTestFeature("TestServer_NodePrice") {
			t.Run("TestServer_NodePrice", func(t *testing.T) {
				t.Log("Execute TestServer_NodePrice")
				test.NodePrice()
			})
		}

		if utils.ShouldTestFeature("TestServer_PodPrice") {
			t.Run("TestServer_PodPrice", func(t *testing.T) {
				t.Log("Execute TestServer_PodPrice")
				test.PodPrice()
			})
		}

		if utils.ShouldTestFeature("TestServer_Cleanup") {
			t.Run("TestServer_Cleanup", func(t *testing.T) {
				t.Log("Execute TestServer_Cleanup")
				test.Cleanup()
			})
		}

	}
}
