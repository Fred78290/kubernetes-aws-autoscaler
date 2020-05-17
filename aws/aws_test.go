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
	CloudInit    interface{}               `json:"cloud-init"`
	SSH          types.AutoScalerServerSSH `json:"ssh"`
	InstanceName string                    `json:"instanceName"`
	InstanceType string                    `json:"instanceType"`
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

func (config *ConfigurationTest) CheckIfIPIsReady(nodename, address string) error {
	command := fmt.Sprintf("hostnamectl set-hostname %s", nodename)

	_, err := utils.Sudo(&config.SSH, address, command)

	return err
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

func Test_getInstanceID(t *testing.T) {
	if testFeature("Test_getInstanceID") {
		config := loadFromJson(confName)

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else {
			if assert.NotNil(t, instance) {
				status, err := instance.Status()

				if assert.NoErrorf(t, err, "Can't get status of VM") {
					t.Logf("The power of vm is:%v", status.Powered)
				}
			}
		}
	}
}

func Test_createInstance(t *testing.T) {
	if testFeature("Test_createInstance") {
		config := loadFromJson(confName)

		_, err := config.Create(0, "test-aws-autoscaler", config.InstanceName, config.InstanceType, config.Disk, config.CloudInit)

		if assert.NoError(t, err, "Can't create VM") {
			t.Logf("VM created")
		}
	}
}

func Test_statusInstance(t *testing.T) {
	if testFeature("Test_statusInstance") {
		config := loadFromJson(confName)

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else {
			status, err := instance.Status()

			if assert.NoError(t, err, "Can't get status VM") {
				t.Logf("The power of vm %s is:%v", config.InstanceName, status.Powered)
			}
		}
	}
}

func Test_waitReadyInstance(t *testing.T) {
	if testFeature("Test_waitReadyInstance") {
		config := loadFromJson(confName)

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else if status, err := instance.Status(); assert.NoError(t, err, "Can't get status on VM") && status.Powered == false {
			if status.Powered == true {
				ipaddr, err := instance.WaitForIP(config)

				if assert.NoError(t, err, "Can't get IP") {
					t.Logf("VM powered with IP:%s", *ipaddr)
				}
			} else {
				t.Logf("VM already powered with IP:%s", status.Address)
			}
		}
	}
}
func Test_powerOnInstance(t *testing.T) {
	if testFeature("Test_powerOnInstance") {
		config := loadFromJson(confName)

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else if status, err := instance.Status(); assert.NoError(t, err, "Can't get status on VM") && status.Powered == false {
			if status.Powered == false {
				err = instance.PowerOn()

				if assert.NoError(t, err, "Can't power on VM") {
					ipaddr, err := instance.WaitForIP(config)

					if assert.NoError(t, err, "Can't get IP") {
						t.Logf("VM powered with IP:%s", *ipaddr)
					}
				}
			} else {
				t.Logf("VM already powered with IP:%s", status.Address)
			}
		}
	}
}

func Test_powerOffInstance(t *testing.T) {
	if testFeature("Test_powerOffInstance") {
		config := loadFromJson(confName)

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else if status, err := instance.Status(); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			err = instance.PowerOff()

			if assert.NoError(t, err, "Can't power off VM") {
				t.Logf("VM shutdown")
			}
		}
	}
}

func Test_shutdownInstance(t *testing.T) {
	if testFeature("Test_shutdownInstance") {
		config := loadFromJson(confName)

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else if status, err := instance.Status(); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			err = instance.ShutdownGuest()

			if assert.NoError(t, err, "Can't power off VM") {
				t.Logf("VM shutdown")
			}
		}
	}
}

func Test_deleteInstance(t *testing.T) {
	if testFeature("Test_deleteInstance") {
		config := loadFromJson(confName)

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else {
			err := instance.Delete()

			if assert.NoError(t, err, "Can't delete VM") {
				t.Logf("VM deleted")
			}
		}
	}
}
