package aws_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/golang/glog"
	"github.com/stretchr/testify/assert"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
)

type ConfigurationTest struct {
	aws.Configuration
	CloudInit interface{}               `json:"cloud-init"`
	SSH       types.AutoScalerServerSSH `json:"ssh"`
	VM        string                    `json:"old-vm"`
	New       *NewVirtualMachineConf    `json:"new-vm"`
}

type NewVirtualMachineConf struct {
	Name       string
	Annotation string
	Memory     int
	CPUS       int
	Disk       int
	Network    *aws.Network
}

var testConfig *ConfigurationTest
var confName = "test.json"

func testFeature(name string) bool {
	if feature := os.Getenv(name); feature != "" {
		return feature != "NO"
	}

	return true
}

func saveToJson(fileName string, config *ConfigurationTest) error {
	file, err := os.Create(fileName)

	if err != nil {
		glog.Errorf("Failed to open file:%s, error:%v", fileName, err)

		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(config)

	if err != nil {
		glog.Errorf("failed to encode config to file:%s, error:%v", fileName, err)

		return err
	}

	return nil
}

func loadFromJson(fileName string) *ConfigurationTest {
	if testConfig == nil {
		file, err := os.Open(fileName)
		if err != nil {
			glog.Fatalf("failed to open config file:%s, error:%v", fileName, err)
		}

		decoder := json.NewDecoder(file)
		err = decoder.Decode(&testConfig)
		if err != nil {
			glog.Fatalf("failed to decode config file:%s, error:%v", fileName, err)
		}
	}

	return testConfig
}

func Test_AuthMethodKey(t *testing.T) {
	if testFeature("Test_AuthMethodKey") {
		config := loadFromJson(confName)

		signer := utils.AuthMethodFromPrivateKeyFile(config.SSH.GetAuthKeys())

		if assert.NotNil(t, signer) {

		}
	}
}

func Test_Sudo(t *testing.T) {
	if testFeature("Test_Sudo") {
		config := loadFromJson(confName)

		out, err := utils.Sudo(&config.SSH, "localhost", "ls")

		if assert.NoError(t, err) {
			t.Log(out)
		}
	}
}

func Test_getVM(t *testing.T) {
	if testFeature("Test_getVM") {
		config := loadFromJson(confName)

		if instanceID, err := config.GetInstanceID(config.New.Name); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.New.Name))
		} else {
			if assert.NotNil(t, instanceID) {
				status, err := vm.Status(ctx)

				if assert.NoErrorf(t, err, "Can't get status of VM") {
					t.Logf("The power of vm is:%v", status.Powered)
				}
			}
		}
	}
}

func Test_createVM(t *testing.T) {
	if testFeature("Test_createVM") {
		config := loadFromJson(confName)
		_, err := config.Create(config.New.Name, config.SSH.GetUserName(), config.SSH.GetAuthKeys(), config.CloudInit, config.New.Network, config.New.Annotation, config.New.Memory, config.New.CPUS, config.New.Disk)

		if assert.NoError(t, err, "Can't create VM") {
			t.Logf("VM created")
		}
	}
}

func Test_statusVM(t *testing.T) {
	if testFeature("Test_statusVM") {
		config := loadFromJson(confName)

		if instanceID, err := config.GetInstanceID(config.New.Name); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.New.Name))
		} else {
			status, err := config.Status(instanceID)

			if assert.NoError(t, err, "Can't get status VM") {
				t.Logf("The power of vm %s is:%v", config.New.Name, status.Powered)
			}
		}
	}
}

func Test_powerOnVM(t *testing.T) {
	if testFeature("Test_powerOnVM") {
		config := loadFromJson(confName)

		if instanceID, err := config.GetInstanceID(config.New.Name); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.New.Name))
		} else if status, err := config.Status(instanceID); assert.NoError(t, err, "Can't get status on VM") && status.Powered == false {
			err = config.PowerOn(instanceID)

			if assert.NoError(t, err, "Can't power on VM") {
				ipaddr, err := config.WaitForIP(config.New.Name)

				if assert.NoError(t, err, "Can't get IP") {
					t.Logf("VM powered with IP:%s", ipaddr)
				}
			}
		}
	}
}

func Test_powerOffVM(t *testing.T) {
	if testFeature("Test_powerOffVM") {
		config := loadFromJson(confName)

		if instanceID, err := config.GetInstanceID(config.New.Name); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.New.Name))
		} else if status, err := config.Status(instanceID); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			err = config.PowerOff(instanceID)

			if assert.NoError(t, err, "Can't power off VM") {
				t.Logf("VM shutdown")
			}
		}
	}
}

func Test_shutdownGuest(t *testing.T) {
	if testFeature("Test_shutdownGuest") {
		config := loadFromJson(confName)

		if instanceID, err := config.GetInstanceID(config.New.Name); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.New.Name))
		} else if status, err := config.Status(instanceID); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			err = config.ShutdownGuest(instanceID)

			if assert.NoError(t, err, "Can't power off VM") {
				t.Logf("VM shutdown")
			}
		}
	}
}

func Test_deleteVM(t *testing.T) {
	if testFeature("Test_deleteVM") {
		config := loadFromJson(confName)

		if instanceID, err := config.GetInstanceID(config.New.Name); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.New.Name))
		} else {
			err := config.Delete(instanceID)

			if assert.NoError(t, err, "Can't delete VM") {
				t.Logf("VM deleted")
			}
		}
	}
}
