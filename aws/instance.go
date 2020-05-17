package aws

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Ec2Instance Running instance
type Ec2Instance struct {
	client       *ec2.EC2
	config       *Configuration
	InstanceName string
	InstanceID   *string
}

// GetEc2Instance return an existing instance from name
func GetEc2Instance(config *Configuration, instanceName string) (*Ec2Instance, error) {
	if client, err := createClient(config); err != nil {
		return nil, err
	} else {
		var result *ec2.DescribeInstancesOutput

		input := &ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name: aws.String("tag:Name"),
					Values: []*string{
						aws.String(instanceName),
					},
				},
			},
		}

		ctx := NewContext(config.Timeout)
		defer ctx.Cancel()

		if result, err = client.DescribeInstancesWithContext(ctx, input); err != nil {
			return nil, err
		}

		if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
			return nil, fmt.Errorf(constantes.ErrVMNotFound, instanceName)
		}

		for _, reservation := range result.Reservations {
			for _, instance := range reservation.Instances {
				if *instance.State.Code != 48 {
					return &Ec2Instance{
						client:       client,
						config:       config,
						InstanceName: instanceName,
						InstanceID:   instance.InstanceId,
					}, nil
				}
			}
		}

		return nil, fmt.Errorf(constantes.ErrVMNotFound, instanceName)
	}
}

// NewEc2Instance create a new instance
func NewEc2Instance(config *Configuration, instanceName string) (*Ec2Instance, error) {
	if client, err := createClient(config); err != nil {
		return nil, err
	} else {
		return &Ec2Instance{
			client:       client,
			config:       config,
			InstanceName: instanceName,
		}, nil
	}
}

func newSession(conf *Configuration) (*session.Session, error) {
	var cred *credentials.Credentials

	if !isNullOrEmpty(conf.AccessKey) && !isNullOrEmpty(conf.SecretKey) && !isNullOrEmpty(conf.Token) {
		cred = credentials.NewStaticCredentials(conf.AccessKey, conf.SecretKey, conf.Token)
	} else if !isNullOrEmpty(conf.AccessKey) && !isNullOrEmpty(conf.SecretKey) {
		cred = credentials.NewStaticCredentials(conf.AccessKey, conf.SecretKey, "")
	} else if !isNullOrEmpty(conf.Profile) {
		cred = credentials.NewSharedCredentials("", conf.Profile)
	} else {
		cred = nil
	}

	config := aws.Config{
		Credentials: cred,
		Region:      aws.String(conf.Region),
	}

	return session.NewSession(&config)
}

func createClient(conf *Configuration) (*ec2.EC2, error) {
	var err error
	var sess *session.Session

	if sess, err = newSession(conf); err != nil {
		return nil, err
	}

	// Create EC2 service client
	return ec2.New(sess, aws.NewConfig().WithLogger(conf).WithLogLevel(aws.LogDebugWithHTTPBody).WithLogLevel(aws.LogDebugWithSigning)), nil
}

func (instance *Ec2Instance) getEc2Instance() (*ec2.Instance, error) {
	var err error
	var result *ec2.DescribeInstancesOutput

	ctx := instance.NewContext()
	defer ctx.Cancel()

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			instance.InstanceID,
		},
	}

	if result, err = instance.client.DescribeInstancesWithContext(ctx, input); err != nil {
		return nil, err
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf(constantes.ErrVMNotFound, instance.InstanceName)
	}

	return result.Reservations[0].Instances[0], nil
}

// NewContext create instance context
func (instance *Ec2Instance) NewContext() *Context {
	return NewContext(instance.config.Timeout)
}

// WaitForIP wait ip a VM by name
func (instance *Ec2Instance) WaitForIP(callback CallbackCheckIPReady) (*string, error) {

	timeout := time.Duration(instance.config.Timeout*1000) * time.Millisecond

	for now := time.Now(); time.Since(now) < timeout; time.Sleep(time.Second) {
		if ec2Instance, err := instance.getEc2Instance(); err != nil {
			return nil, err
		} else {
			var code int64 = *ec2Instance.State.Code

			if code == 16 {
				var address *string

				if instance.config.Network.UsePublicIPAddress {
					address = ec2Instance.PublicIpAddress
				} else {
					address = ec2Instance.PrivateIpAddress
				}

				for t := time.Now(); time.Since(t) < timeout; time.Sleep(time.Second) {
					if err = callback.CheckIfIPIsReady(instance.InstanceName, *address); err == nil {
						return address, nil
					}
				}

				return address, fmt.Errorf(constantes.ErrWaitIPTimeout, instance.InstanceName, instance.config.Timeout)
			}

			if code != 0 {
				return nil, fmt.Errorf(constantes.ErrWrongStateMachine, *ec2Instance.State.Name, instance.InstanceName)
			}
		}
	}

	return nil, fmt.Errorf(constantes.ErrWaitIPTimeout, instance.InstanceName, instance.config.Timeout)
}

// WaitForPowered wait ip a VM by name
func (instance *Ec2Instance) WaitForPowered() error {

	timeout := time.Duration(instance.config.Timeout*1000) * time.Millisecond

	for now := time.Now(); time.Since(now) < timeout; time.Sleep(time.Second) {
		if ec2Instance, err := instance.getEc2Instance(); err != nil {
			return err
		} else {

			var code int64 = *ec2Instance.State.Code

			if code == 16 {
				return nil
			}

			if code != 0 {
				return fmt.Errorf(constantes.ErrWrongStateMachine, *ec2Instance.State.Name, instance.InstanceName)
			}
		}
	}

	return fmt.Errorf(constantes.ErrWaitIPTimeout, instance.InstanceName, instance.config.Timeout)
}

// Create will create a named VM not powered
// memory and disk are in megabytes
func (instance *Ec2Instance) Create(nodeIndex int, nodeGroup, instanceType, userData string, disk int) error {
	var err error
	var result *ec2.Reservation

	ctx := instance.NewContext()
	defer ctx.Cancel()

	input := &ec2.RunInstancesInput{
		InstanceType:                      aws.String(instanceType),
		ImageId:                           aws.String(instance.config.ImageID),
		KeyName:                           aws.String(instance.config.KeyName),
		InstanceInitiatedShutdownBehavior: aws.String(ec2.ShutdownBehaviorStop),
		MaxCount:                          aws.Int64(1),
		MinCount:                          aws.Int64(1),
		UserData:                          aws.String(userData),
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Arn: &instance.config.IamRole,
		},
	}

	instanceTags := append(make([]*ec2.Tag, 0, len(instance.config.Tags)+3), &ec2.Tag{
		Key:   aws.String("Name"),
		Value: aws.String(instance.InstanceName),
	})

	instanceTags = append(instanceTags, &ec2.Tag{
		Key:   aws.String("NodeGroup"),
		Value: aws.String(nodeGroup),
	})

	instanceTags = append(instanceTags, &ec2.Tag{
		Key:   aws.String("NodeIndex"),
		Value: aws.String(strconv.Itoa(nodeIndex)),
	})

	// Add tags
	if instance.config.Tags != nil && len(instance.config.Tags) > 0 {
		for _, tag := range instance.config.Tags {
			instanceTags = append(instanceTags, &ec2.Tag{
				Key:   aws.String(tag.Key),
				Value: aws.String(tag.Value),
			})
		}
	}

	input.TagSpecifications = []*ec2.TagSpecification{
		{
			ResourceType: aws.String(ec2.ResourceTypeInstance),
			Tags:         instanceTags,
		},
	}

	if len(instance.config.Network.ENI) > 1 || instance.config.Network.UsePublicIPAddress {
		interfaces := make([]*ec2.InstanceNetworkInterfaceSpecification, len(instance.config.Network.ENI))

		for index, eni := range instance.config.Network.ENI {
			inf := &ec2.InstanceNetworkInterfaceSpecification{
				AssociatePublicIpAddress: &eni.PublicIP,
				DeleteOnTermination:      aws.Bool(true),
				Description:              aws.String(instance.InstanceName),
				DeviceIndex:              aws.Int64(int64(index)),
				SubnetId:                 aws.String(eni.SubnetID),
				Groups: []*string{
					aws.String(eni.SecurityGroupID),
				},
			}
			interfaces[index] = inf
		}

		if len(interfaces) > 0 {
			input.NetworkInterfaces = interfaces
		}
	} else if len(instance.config.Network.ENI) > 0 {
		input.SubnetId = aws.String(instance.config.Network.ENI[0].SubnetID)
		input.SecurityGroupIds = []*string{
			aws.String(instance.config.Network.ENI[0].SecurityGroupID),
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

	if result, err = instance.client.RunInstancesWithContext(ctx, input); err != nil {
		return err
	}

	instance.InstanceID = result.Instances[0].InstanceId

	return nil
}

// Delete a VM by name
func (instance *Ec2Instance) Delete() error {
	ctx := instance.NewContext()
	defer ctx.Cancel()

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			instance.InstanceID,
		},
	}

	if _, err := instance.client.TerminateInstancesWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

// PowerOn power on a VM by name
func (instance *Ec2Instance) PowerOn() error {
	ctx := instance.NewContext()
	defer ctx.Cancel()

	input := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			instance.InstanceID,
		},
	}

	if _, err := instance.client.StartInstancesWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

func (instance *Ec2Instance) powerOff(force bool) error {
	ctx := instance.NewContext()
	defer ctx.Cancel()

	input := &ec2.StopInstancesInput{
		Force: &force,
		InstanceIds: []*string{
			instance.InstanceID,
		},
	}

	if _, err := instance.client.StopInstancesWithContext(ctx, input); err != nil {
		return err
	}

	return nil
}

// PowerOff power off a VM by name
func (instance *Ec2Instance) PowerOff() error {
	return instance.powerOff(true)
}

// ShutdownGuest power off a VM by name
func (instance *Ec2Instance) ShutdownGuest() error {
	return instance.powerOff(false)
}

// Status return the current status of VM by name
func (instance *Ec2Instance) Status() (*Status, error) {

	if ec2Instance, err := instance.getEc2Instance(); err != nil {
		return nil, err
	} else {

		var address *string

		code := ec2Instance.State.Code

		if code == nil || *code == 48 {
			return nil, fmt.Errorf("EC2 Instance %s is terminated", instance.InstanceName)
		} else if *code == 16 || *code == 0 {
			if instance.config.Network.UsePublicIPAddress {
				address = ec2Instance.PublicIpAddress
			} else {
				address = ec2Instance.PrivateIpAddress
			}

			return &Status{
				Address: *address,
				Powered: *code == 16 || *code == 0,
			}, nil
		} else {
			return &Status{}, nil
		}
	}
}
