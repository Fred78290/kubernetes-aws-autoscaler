package aws_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	glog "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"
)

type ConfigurationTest struct {
	aws.Configuration
	SSH          types.AutoScalerServerSSH `json:"ssh"`
	InstanceName string                    `json:"instanceName"`
	InstanceType string                    `json:"instanceType"`
	inited       bool
}

var testConfig ConfigurationTest

func getConfFile() string {
	if config := os.Getenv("TEST_CONFIG"); config != "" {
		return config
	}

	return "../test/local_aws.json"
}

func loadFromJson(fileName string) *ConfigurationTest {
	if !testConfig.inited {
		if configStr, err := os.ReadFile(fileName); err != nil {
			glog.Fatalf("failed to open config file:%s, error:%v", fileName, err)
		} else {
			err = json.Unmarshal(configStr, &testConfig)

			if err != nil {
				glog.Fatalf("failed to decode config file:%s, error:%v", fileName, err)
			}
		}
	}

	return &testConfig
}

func (config *ConfigurationTest) WaitSSHReady(nodename, address string) error {
	return utils.PollImmediate(time.Second, time.Duration(config.SSH.WaitSshReadyInSeconds)*time.Second, func() (done bool, err error) {
		// Set hostname
		if _, err := utils.Sudo(&config.SSH, address, time.Second, fmt.Sprintf("hostnamectl set-hostname %s", nodename)); err != nil {
			if strings.HasSuffix(err.Error(), "connection refused") || strings.HasSuffix(err.Error(), "i/o timeout") {
				return false, nil
			}

			return false, err
		}
		return true, nil
	})
}

func Test_AuthMethodKey(t *testing.T) {
	if utils.ShouldTestFeature("Test_AuthMethodKey") {
		config := loadFromJson(getConfFile())

		_, err := utils.AuthMethodFromPrivateKeyFile(config.SSH.GetAuthKeys())

		if assert.NoError(t, err) {
			t.Log("OK")
		}
	}
}

func Test_Sudo(t *testing.T) {
	if utils.ShouldTestFeature("Test_Sudo") {
		config := loadFromJson(getConfFile())

		out, err := utils.Sudo(&config.SSH, "localhost", 10, "ls")

		if assert.NoError(t, err) {
			t.Log(out)
		}
	}
}

func Test_getInstanceID(t *testing.T) {
	if utils.ShouldTestFeature("Test_getInstanceID") {
		config := loadFromJson(getConfFile())

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			if !strings.HasPrefix(err.Error(), "unable to find VM:") {
				assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
			}
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
	if utils.ShouldTestFeature("Test_createInstance") {
		config := loadFromJson(getConfFile())

		_, err := config.Create(0, "test-aws-autoscaler", config.InstanceName, config.InstanceType, config.DiskType, config.DiskSize, nil, nil)

		if assert.NoError(t, err, "Can't create VM") {
			t.Logf("VM created")
		}
	}
}

func Test_statusInstance(t *testing.T) {
	if utils.ShouldTestFeature("Test_statusInstance") {
		config := loadFromJson(getConfFile())

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

func Test_waitForPowered(t *testing.T) {
	if utils.ShouldTestFeature("Test_waitForPowered") {
		config := loadFromJson(getConfFile())

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else if status, err := instance.Status(); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			err := instance.WaitForPowered()

			if assert.NoError(t, err, "Can't WaitForPowered") {
				t.Log("VM powered")
			}
		} else {
			t.Log("VM is not powered")
		}
	}
}

func Test_waitForIP(t *testing.T) {
	if utils.ShouldTestFeature("Test_waitForIP") {
		config := loadFromJson(getConfFile())

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else if status, err := instance.Status(); assert.NoError(t, err, "Can't get status on VM") && status.Powered {
			ipaddr, err := instance.WaitForIP(config)

			if assert.NoError(t, err, "Can't get IP") {
				t.Logf("VM powered with IP:%s", *ipaddr)
			}
		} else {
			t.Log("VM is not powered")
		}
	}
}

func Test_powerOnInstance(t *testing.T) {
	if utils.ShouldTestFeature("Test_powerOnInstance") {
		config := loadFromJson(getConfFile())

		if instance, err := config.GetInstanceID(config.InstanceName); err != nil {
			assert.NoError(t, err, fmt.Sprintf("Can't find ec2 instance named:%s", config.InstanceName))
		} else if status, err := instance.Status(); assert.NoError(t, err, "Can't get status on VM") {
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
	if utils.ShouldTestFeature("Test_powerOffInstance") {
		config := loadFromJson(getConfFile())

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
	if utils.ShouldTestFeature("Test_shutdownInstance") {
		config := loadFromJson(getConfFile())

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
	if utils.ShouldTestFeature("Test_deleteInstance") {
		config := loadFromJson(getConfFile())

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
