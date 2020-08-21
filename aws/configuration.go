package aws

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"gopkg.in/yaml.v2"
)

// VirtualMachinePowerState alias string
type VirtualMachinePowerState string

const (
	// VirtualMachinePowerStatePoweredOff state
	VirtualMachinePowerStatePoweredOff = VirtualMachinePowerState("poweredOff")

	// VirtualMachinePowerStatePoweredOn state
	VirtualMachinePowerStatePoweredOn = VirtualMachinePowerState("poweredOn")

	// VirtualMachinePowerStateSuspended state
	VirtualMachinePowerStateSuspended = VirtualMachinePowerState("suspended")
)

// Configuration declares aws connection info
type Configuration struct {
	AccessKey string   `json:"accessKey"`
	SecretKey string   `json:"secretKey"`
	Token     string   `json:"token"`
	Profile   string   `json:"profile"`
	Region    string   `json:"region"`
	Timeout   float64  `json:"timeout"`
	ImageID   string   `json:"ami"`
	IamRole   string   `json:"iam-role-arn"`
	KeyName   string   `json:"keyName"`
	Tags      []Tag    `json:"tags"`
	Network   *Network `json:"network"`
	Disk      int      `json:"diskSize"`
}

// Tag aws tag
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Network declare network configuration
type Network struct {
	ZoneID          *string `json:"route53"`
	PrivateZoneName *string `json:"privateZoneName"`
	ENI             []Eni   `json:"eni"`
}

// Eni decalre ENI interface
type Eni struct {
	SubnetID        string `json:"subnet"`
	SecurityGroupID string `json:"securityGroup"`
	PublicIP        bool   `json:"publicIP"`
}

// Status shortened vm status
type Status struct {
	Address string
	Powered bool
}

// CallbackCheckIPReady callback to test if IP is up
type CallbackCheckIPReady interface {
	CheckIfIPIsReady(name, address string) error
}

func isNullOrEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

func encodeCloudInit(object interface{}) string {
	var out bytes.Buffer

	fmt.Fprintln(&out, "#cloud-config")

	b, _ := yaml.Marshal(object)

	fmt.Fprintln(&out, string(b))

	return base64.StdEncoding.EncodeToString(out.Bytes())
}

// Copy Make a deep copy from src into dst.
func Copy(dst interface{}, src interface{}) error {
	if dst == nil {
		return fmt.Errorf("dst cannot be nil")
	}

	if src == nil {
		return fmt.Errorf("src cannot be nil")
	}

	bytes, err := json.Marshal(src)

	if err != nil {
		return fmt.Errorf("Unable to marshal src: %s", err)
	}

	err = json.Unmarshal(bytes, dst)

	if err != nil {
		return fmt.Errorf("Unable to unmarshal into dst: %s", err)
	}

	return nil
}

// Log logging
func (conf *Configuration) Log(args ...interface{}) {
	glog.Infoln(args...)
}

// GetInstanceID return aws instance id from named ec2 instance
func (conf *Configuration) GetInstanceID(name string) (*Ec2Instance, error) {
	return GetEc2Instance(conf, name)
}

// Create will create a named VM not powered
// memory and disk are in megabytes
func (conf *Configuration) Create(nodeIndex int, nodeGroup, name, instanceType string, disk int, userData interface{}) (*Ec2Instance, error) {
	var err error
	var instance *Ec2Instance

	if instance, err = NewEc2Instance(conf, name); err != nil {
		return nil, err
	}

	if err = instance.Create(nodeIndex, nodeGroup, instanceType, encodeCloudInit(userData), disk); err != nil {
		return nil, err
	}

	return instance, nil
}
