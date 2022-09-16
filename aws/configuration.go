package aws

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	glog "github.com/sirupsen/logrus"
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
	AccessKey string        `json:"accessKey"`
	SecretKey string        `json:"secretKey"`
	Token     string        `json:"token"`
	Profile   string        `json:"profile"`
	Region    string        `json:"region"`
	Timeout   time.Duration `json:"timeout"`
	ImageID   string        `json:"ami"`
	IamRole   string        `json:"iam-role-arn"`
	KeyName   string        `json:"keyName"`
	Tags      []Tag         `json:"tags"`
	Network   *Network      `json:"network"`
	DiskType  string        `default:"standard" json:"diskType"`
	DiskSize  int           `default:"10" json:"diskSize"`
}

// Tag aws tag
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Network declare network configuration
type Network struct {
	ZoneID          *string            `json:"route53"`
	PrivateZoneName *string            `json:"privateZoneName"`
	AccessKey       *string            `json:"accessKey"`
	SecretKey       *string            `json:"secretKey"`
	Token           *string            `json:"token"`
	Profile         *string            `json:"profile"`
	Region          *string            `json:"region"`
	ENI             []NetworkInterface `json:"eni"`
}

// NetworkInterface declare ENI interface
type NetworkInterface struct {
	SubnetsID       interface{} `json:"subnets"`
	SecurityGroupID string      `json:"securityGroup"`
	PublicIP        bool        `json:"publicIP"`
}

// UserDefinedNetworkInterface declare a network interface interface overriding default Eni
type UserDefinedNetworkInterface struct {
	NetworkInterfaceID string `json:"networkInterfaceId"`
	SubnetID           string `json:"subnets"`
	SecurityGroupID    string `json:"securityGroup"`
	PrivateAddress     string `json:"privateAddress,omitempty"`
	PublicIP           bool   `json:"publicIP"`
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

func randomNumberInRange(min, max int) int {
	return rand.Intn(max-min) + min
}

func isNullOrEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
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
		return fmt.Errorf("unable to marshal src: %s", err)
	}

	err = json.Unmarshal(bytes, dst)

	if err != nil {
		return fmt.Errorf("unable to unmarshal into dst: %s", err)
	}

	return nil
}

func (eni *NetworkInterface) GetRandomSubnetsID() *string {
	var str string
	s := reflect.ValueOf(eni.SubnetsID)

	switch reflect.TypeOf(eni.SubnetsID).Kind() {
	case reflect.String:
		str = s.String()
	case reflect.Slice:
		str = fmt.Sprintf("%v", s.Index(randomNumberInRange(0, (s.Len()*2)-1)))
	}

	if len(str) > 0 {
		return aws.String(str)
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

// GetRoute53AccessKey return route53 access key or default
func (conf *Configuration) GetRoute53AccessKey() string {
	if conf.Network.AccessKey != nil && *conf.Network.AccessKey != "" {
		return *conf.Network.AccessKey
	}

	return conf.AccessKey
}

// GetRoute53SecretKey return route53 secret key or default
func (conf *Configuration) GetRoute53SecretKey() string {
	if conf.Network.SecretKey != nil && *conf.Network.SecretKey != "" {
		return *conf.Network.SecretKey
	}

	return conf.SecretKey
}

// GetRoute53AccessToken return route53 token or default
func (conf *Configuration) GetRoute53AccessToken() string {
	if conf.Network.Token != nil && *conf.Network.Token != "" {
		return *conf.Network.Token
	}

	return conf.Token
}

// GetRoute53Profile return route53 profile or default
func (conf *Configuration) GetRoute53Profile() string {
	if conf.Network.Profile != nil && *conf.Network.Profile != "" {
		return *conf.Network.Profile
	}

	return conf.Profile
}

// GetRoute53Profile return route53 region or default
func (conf *Configuration) GetRoute53Region() string {
	if conf.Network.Region != nil && *conf.Network.Region != "" {
		return *conf.Network.Region
	}

	return conf.Region
}

// Create will create a named VM not powered
// memory and disk are in megabytes
func (conf *Configuration) Create(nodeIndex int, nodeGroup, name, instanceType string, diskType string, diskSize int, userData *string, desiredENI *UserDefinedNetworkInterface) (*Ec2Instance, error) {
	var err error
	var instance *Ec2Instance

	if instance, err = NewEc2Instance(conf, name); err != nil {
		return nil, err
	}

	if err = instance.Create(nodeIndex, nodeGroup, instanceType, userData, diskType, diskSize, desiredENI); err != nil {
		return nil, err
	}

	return instance, nil
}
