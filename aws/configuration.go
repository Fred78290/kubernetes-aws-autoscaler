package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/aws/aws-sdk-go/aws/session"
	"gopkg.in/yaml.v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ec2"
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
	AccessKey    string        `json:"accessKey"`
	SecretKey    string        `json:"secretKey"`
	Token        string        `json:"token"`
	Profile      string        `json:"profile"`
	Region       string        `json:"region"`
	Timeout      time.Duration `json:"timeout"`
	ImageID      string        `json:"ami"`
	InstanceType string        `json:"instance-type"`
	IamRole      string        `json:"iam-role"`
	KeyName      string        `json:"keyName"`
	Tags         []Tag         `json:"tags"`
	Network      *Network      `json:"network"`
	InstanceName string        `json:"instance-name"`
	ec2          *ec2.EC2
}

// Tag aws tag
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Network declare network configuration
type Network struct {
	UsePublicIPAddress bool  `json:"usePublicIP"`
	ENI                []Eni `json:"eni"`
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
	return len(strings.TrimSpace(s)) > 0
}

func encodeCloudInit(object interface{}) string {
	var out bytes.Buffer

	fmt.Fprintln(&out, "#cloud-init")

	b, _ := yaml.Marshal(object)

	fmt.Fprintln(&out, string(b))

	return out.String()
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

func (conf *Configuration) newSession() (*session.Session, error) {
	var cred *credentials.Credentials

	if !isNullOrEmpty(conf.AccessKey) && !isNullOrEmpty(conf.SecretKey) && !isNullOrEmpty(conf.Token) {
		cred = credentials.NewStaticCredentials(conf.AccessKey, conf.SecretKey, conf.Token)
	} else if !isNullOrEmpty(conf.AccessKey) && !isNullOrEmpty(conf.SecretKey) {
		cred = credentials.NewStaticCredentials(conf.AccessKey, conf.SecretKey, "")
	} else {
		cred = nil
	}

	config := aws.Config{
		Credentials: cred,
		Region:      aws.String(conf.Region),
	}

	return session.NewSession(&config)
}

func (conf *Configuration) getEC2() (*ec2.EC2, error) {
	if conf.ec2 == nil {
		var err error
		var sess *session.Session

		if sess, err = conf.newSession(); err != nil {
			return nil, err
		}

		// Create EC2 service client
		conf.ec2 = ec2.New(sess)
	}

	return conf.ec2, nil
}

// GetInstanceID return aws instance id from named ec2 instance
func (conf *Configuration) GetInstanceID(name string) (*string, error) {
	var err error
	var client *ec2.EC2
	var result *ec2.DescribeInstancesOutput

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	if client, err = conf.getEC2(); err != nil {
		return nil, err
	}

	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String(name),
				},
			},
		},
	}

	if result, err = client.DescribeInstancesWithContext(ctx, input); err != nil {
		return nil, err
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf(constantes.ErrVMNotFound, name)
	}

	return result.Reservations[0].Instances[0].InstanceId, nil
}

// Clone duplicate the conf, change ip address in network config if needed
func (conf *Configuration) Clone(nodeName string, nodeIndex int) (*Configuration, error) {

	var dup Configuration

	if err := Copy(&dup, conf); err != nil {
		return nil, err
	}

	dup.InstanceName = ""

	return &dup, nil
}

// WaitForIP wait ip a VM by name
func (conf *Configuration) WaitForIP(instanceID string, callback CallbackCheckIPReady) (*string, error) {
	var err error
	var client *ec2.EC2
	var result *ec2.DescribeInstancesOutput

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	if client, err = conf.getEC2(); err != nil {
		return nil, err
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}

	timeout := conf.Timeout

	for timeout > 0 {
		if result, err = client.DescribeInstancesWithContext(ctx, input); err != nil {
			return nil, err
		}

		if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
			return nil, fmt.Errorf(constantes.ErrVMNotFound, conf.InstanceName)
		}

		var code int64 = *result.Reservations[0].Instances[0].State.Code

		if code == 16 {
			var address *string

			if conf.Network.UsePublicIPAddress {
				address = result.Reservations[0].Instances[0].PublicIpAddress
			} else {
				address = result.Reservations[0].Instances[0].PrivateIpAddress
			}

			//command := fmt.Sprintf("hostnamectl set-hostname %s", conf.InstanceName)
			count := conf.Timeout

			for count > 0 {
				if err = callback.CheckIfIPIsReady(conf.InstanceName, *address); err == nil {
					return address, nil
				}

				time.Sleep(time.Second)

				count--
			}

			return address, fmt.Errorf(constantes.ErrWaitIPTimeout, conf.InstanceName, conf.Timeout)
		}

		if code != 0 {
			return nil, fmt.Errorf(constantes.ErrWrongStateMachine, result.Reservations[0].Instances[0].State.Name, conf.InstanceName)
		}

		time.Sleep(time.Second)

		timeout--
	}

	return nil, fmt.Errorf(constantes.ErrWaitIPTimeout, conf.InstanceName, conf.Timeout)
}

// WaitForPowered wait ip a VM by name
func (conf *Configuration) WaitForPowered(instanceID string) error {
	var err error
	var client *ec2.EC2
	var result *ec2.DescribeInstancesOutput

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	if client, err = conf.getEC2(); err != nil {
		return err
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}

	count := conf.Timeout

	for count > 0 {
		if result, err = client.DescribeInstancesWithContext(ctx, input); err != nil {
			return err
		}

		if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
			return fmt.Errorf(constantes.ErrVMNotFound, conf.InstanceName)
		}

		var code int64 = *result.Reservations[0].Instances[0].State.Code

		if code == 16 {
			return nil
		}

		if code != 0 {
			return fmt.Errorf(constantes.ErrWrongStateMachine, result.Reservations[0].Instances[0].State.Name, conf.InstanceName)
		}

		time.Sleep(time.Second)

		count--
	}

	return fmt.Errorf(constantes.ErrWaitIPTimeout, conf.InstanceName, conf.Timeout)
}

// Create will create a named VM not powered
// memory and disk are in megabytes
func (conf *Configuration) Create(name, userName, ami string, disk int, userData interface{}) (string, error) {
	var err error
	var client *ec2.EC2
	var result *ec2.Reservation
	var count int64 = 1

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	input := &ec2.RunInstancesInput{
		InstanceType:                      aws.String(conf.InstanceType),
		ImageId:                           aws.String(conf.ImageID),
		KeyName:                           aws.String(conf.KeyName),
		InstanceInitiatedShutdownBehavior: aws.String(ec2.ShutdownBehaviorStop),
		MaxCount:                          aws.Int64(count),
		MinCount:                          aws.Int64(count),
		UserData:                          aws.String(encodeCloudInit(userData)),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Arn: &conf.IamRole,
		},
	}

	instanceTags := append(make([]*ec2.Tag, 0, len(conf.Tags)+1), &ec2.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(name),
	})

	// Add tags
	if conf.Tags != nil && len(conf.Tags) > 0 {
		for _, tag := range conf.Tags {
			instanceTags = append(instanceTags, &ec2.Tag{
				Key:   aws.String(tag.Key),
				Value: aws.String(tag.Value),
			})
		}
	}

	input.TagSpecifications = []*ec2.TagSpecification{
		&ec2.TagSpecification{
			ResourceType: aws.String(ec2.ResourceTypeInstance),
			Tags:         instanceTags,
		},
	}

	if len(conf.Network.ENI) > 1 || conf.Network.UsePublicIPAddress {
		interfaces := make([]*ec2.InstanceNetworkInterfaceSpecification, len(conf.Network.ENI))

		for index, eni := range conf.Network.ENI {
			inf := &ec2.InstanceNetworkInterfaceSpecification{
				AssociatePublicIpAddress: &eni.PublicIP,
				DeleteOnTermination:      aws.Bool(true),
				Description:              aws.String(name),
				DeviceIndex:              aws.Int64(int64(index)),
				SubnetId:                 aws.String(eni.SubnetID),
				Groups: []*string{
					aws.String(eni.SecurityGroupID),
				},
			}
			interfaces[index] = inf
		}

		input.NetworkInterfaces = interfaces

	} else {
		input.SubnetId = aws.String(conf.Network.ENI[0].SubnetID)
		input.SecurityGroupIds = []*string{
			aws.String(conf.Network.ENI[0].SecurityGroupID),
		}
	}

	if disk > 0 {
		ebs := &ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/sda1"),
			Ebs: &ec2.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				VolumeSize:          aws.Int64(int64(disk)),
			},
		}

		input.BlockDeviceMappings = []*ec2.BlockDeviceMapping{
			ebs,
		}
	}
	if client, err = conf.getEC2(); err != nil {
		return "", err
	} else if result, err = client.RunInstancesWithContext(ctx, input); err != nil {
		return "", err
	}

	return *result.Instances[0].InstanceId, nil
}

// Delete a VM by name
func (conf *Configuration) Delete(instanceID string) error {
	var err error
	var client *ec2.EC2

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}

	if client, err = conf.getEC2(); err != nil {
		return err
	} else if _, err = client.TerminateInstancesWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

// PowerOn power on a VM by name
func (conf *Configuration) PowerOn(instanceID string) error {
	var err error
	var client *ec2.EC2

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	input := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}

	if client, err = conf.getEC2(); err != nil {
		return err
	} else if _, err = client.StartInstancesWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

func (conf *Configuration) powerOff(instanceID string, force bool) error {
	var err error
	var client *ec2.EC2

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	input := &ec2.StopInstancesInput{
		Force: &force,
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}

	if client, err = conf.getEC2(); err != nil {
		return err
	} else if _, err = client.StopInstancesWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

// PowerOff power off a VM by name
func (conf *Configuration) PowerOff(instanceID string) error {
	return conf.powerOff(instanceID, true)
}

// ShutdownGuest power off a VM by name
func (conf *Configuration) ShutdownGuest(instanceID string) error {
	return conf.powerOff(instanceID, false)
}

// Status return the current status of VM by name
func (conf *Configuration) Status(instanceID string) (*Status, error) {
	var err error
	var client *ec2.EC2
	var result *ec2.DescribeInstancesOutput
	var address *string

	ctx := NewContext(conf.Timeout)
	defer ctx.Cancel()

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}

	if client, err = conf.getEC2(); err != nil {
		return nil, err
	} else if result, err = client.DescribeInstancesWithContext(ctx, input); err != nil {
		return nil, err
	}

	code := result.Reservations[0].Instances[0].State.Code

	if conf.Network.UsePublicIPAddress {
		address = result.Reservations[0].Instances[0].PublicIpAddress
	} else {
		address = result.Reservations[0].Instances[0].PrivateIpAddress
	}

	status := &Status{
		Address: *address,
		Powered: *code == 16 || *code == 0,
	}

	return status, nil
}
