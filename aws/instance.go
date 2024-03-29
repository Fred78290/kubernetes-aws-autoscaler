package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	glog "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-aws-autoscaler/context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
)

const (
	route53_UpsertCmd = "UPSERT"
	route53_DeleteCmd = "DELETE"
)

// Ec2Instance Running instance
type Ec2Instance struct {
	client       *ec2.EC2
	config       *Configuration
	InstanceName string
	InstanceID   *string
	Region       *string
	Zone         *string
	AddressIP    *string
}

var phEC2Client *ec2.EC2

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

		ctx := context.NewContext(config.Timeout)
		defer ctx.Cancel()

		if result, err = client.DescribeInstancesWithContext(ctx, input); err != nil {
			return nil, err
		}

		if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
			return nil, fmt.Errorf(constantes.ErrVMNotFound, instanceName)
		}

		for _, reservation := range result.Reservations {
			for _, instance := range reservation.Instances {
				// Assume EC2 shutting-down is terminated after
				if *instance.State.Code != 48 && *instance.State.Code != 32 {
					var address *string

					if instance.PublicIpAddress != nil {
						address = instance.PublicIpAddress
					} else {
						address = instance.PrivateIpAddress
					}

					return &Ec2Instance{
						client:       client,
						config:       config,
						InstanceName: instanceName,
						InstanceID:   instance.InstanceId,
						Region:       &config.Region,
						Zone:         instance.Placement.AvailabilityZone,
						AddressIP:    address,
					}, nil
				}
			}
		}

		return nil, fmt.Errorf(constantes.ErrVMNotFound, instanceName)
	}
}

func userHomeDir() string {
	if runtime.GOOS == "windows" { // Windows
		return os.Getenv("USERPROFILE")
	}

	// *nix
	return os.Getenv("HOME")
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

func credentialsFileExists(filename string) bool {
	if isNullOrEmpty(filename) {
		if filename = os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); isNullOrEmpty(filename) {
			filename = filepath.Join(userHomeDir(), ".aws", "credentials")
		}
	}

	if _, err := os.Stat(filename); err != nil {
		return false
	}

	return true
}

func isAwsProfileValid(filename, profile string) bool {
	if isNullOrEmpty(profile) {
		return false
	}

	return credentialsFileExists(filename)
}

func newSessionWithOptions(accessKey, secretKey, token, filename, profile, region string) (*session.Session, error) {
	var cred *credentials.Credentials

	if isAwsProfileValid(filename, profile) {
		cred = credentials.NewSharedCredentials(filename, profile)
	} else if !isNullOrEmpty(accessKey) && !isNullOrEmpty(secretKey) {
		cred = credentials.NewStaticCredentials(accessKey, secretKey, token)
	} else {
		cred = nil
	}

	config := aws.Config{
		Credentials: cred,
		Region:      aws.String(region),
	}

	return session.NewSession(&config)
}

func newSession(conf *Configuration) (*session.Session, error) {
	return newSessionWithOptions(conf.AccessKey, conf.SecretKey, conf.Token, conf.Filename, conf.Profile, conf.Region)
}

func createClient(conf *Configuration) (*ec2.EC2, error) {
	if phEC2Client == nil {
		var err error
		var sess *session.Session

		if sess, err = newSession(conf); err != nil {
			return nil, err
		}

		// Create EC2 service client
		if glog.GetLevel() >= glog.DebugLevel {
			phEC2Client = ec2.New(sess, aws.NewConfig().WithLogger(conf).WithLogLevel(aws.LogDebugWithHTTPBody).WithLogLevel(aws.LogDebugWithSigning))
		} else {
			phEC2Client = ec2.New(sess)
		}
	}

	return phEC2Client, nil
}

func (instance *Ec2Instance) getInstanceID() string {
	if instance.InstanceID == nil {
		return "<UNDEFINED>"
	} else {
		return *instance.InstanceID
	}
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
func (instance *Ec2Instance) NewContext() *context.Context {
	return context.NewContext(instance.config.Timeout)
}

func (instance *Ec2Instance) NewContextWithTimeout(timeout time.Duration) *context.Context {
	return context.NewContext(timeout)
}

func (instance *Ec2Instance) pollImmediate(interval, timeout time.Duration, condition wait.ConditionFunc) error {
	if timeout == 0 {
		return wait.PollImmediateInfinite(interval, condition)
	} else {
		return wait.PollImmediate(interval, timeout, condition)
	}
}

// WaitForIP wait ip a VM by name
func (instance *Ec2Instance) WaitForIP(callback CallbackWaitSSHReady) (*string, error) {
	glog.Debugf("WaitForIP: instance %s id (%s)", instance.InstanceName, instance.getInstanceID())

	if err := instance.pollImmediate(time.Second, instance.config.Timeout*time.Second, func() (bool, error) {
		var err error
		var ec2Instance *ec2.Instance

		if ec2Instance, err = instance.getEc2Instance(); err != nil {
			return false, err
		}

		code := *ec2Instance.State.Code

		if code == 16 {

			if ec2Instance.PublicIpAddress != nil {
				instance.AddressIP = ec2Instance.PublicIpAddress
			} else {
				instance.AddressIP = ec2Instance.PrivateIpAddress
			}

			glog.Debugf("WaitForIP: instance %s id (%s), using IP:%s", instance.InstanceName, instance.getInstanceID(), *instance.AddressIP)

			if err = callback.WaitSSHReady(instance.InstanceName, *instance.AddressIP); err != nil {
				return false, err
			}

			return true, nil
		} else if code != 0 {
			return false, fmt.Errorf(constantes.ErrWrongStateMachine, *ec2Instance.State.Name, instance.InstanceName)
		}

		return false, nil
	}); err != nil {
		return nil, err
	}

	return instance.AddressIP, nil
}

// WaitForPowered wait ip a VM by name
func (instance *Ec2Instance) WaitForPowered() error {

	glog.Debugf("WaitForPowered: instance %s id (%s)", instance.InstanceName, instance.getInstanceID())

	return instance.pollImmediate(time.Second, instance.config.Timeout*time.Second, func() (bool, error) {
		var err error
		var ec2Instance *ec2.Instance

		if ec2Instance, err = instance.getEc2Instance(); err != nil {
			glog.Debugf("WaitForPowered: instance %s id (%s), got an error %v", instance.InstanceName, instance.getInstanceID(), err)

			return false, err
		}

		var code int64 = *ec2Instance.State.Code

		if code != 16 {
			if code == 0 {
				return false, nil
			}

			glog.Debugf("WaitForPowered: instance %s id (%s), unexpected state: %d", instance.InstanceName, instance.getInstanceID(), code)

			return false, fmt.Errorf(constantes.ErrWrongStateMachine, *ec2Instance.State.Name, instance.InstanceName)
		}

		glog.Debugf("WaitForPowered: ready instance %s id (%s)", instance.InstanceName, instance.getInstanceID())

		return true, nil
	})
}

func (instance *Ec2Instance) buildNetworkInterfaces(nodeIndex int, desiredENI *UserDefinedNetworkInterface) ([]*ec2.InstanceNetworkInterfaceSpecification, error) {
	var err error

	if desiredENI != nil {
		var privateIPAddress *string
		var subnetID *string
		var securityGroup *string
		var deleteOnTermination bool
		var networkInterfaceId *string

		if len(desiredENI.PrivateAddress) > 0 {
			privateIPAddress = aws.String(desiredENI.PrivateAddress)

			var output *ec2.DescribeNetworkInterfacesOutput
			var input = ec2.DescribeNetworkInterfacesInput{
				Filters: []*ec2.Filter{
					{
						Name: aws.String("addresses.private-ip-address"),
						Values: []*string{
							privateIPAddress,
						},
					},
				},
			}

			// Check if associated with ENI
			if output, err = instance.client.DescribeNetworkInterfaces(&input); err != nil {
				if len(output.NetworkInterfaces) > 0 {
					privateIPAddress = nil
					desiredENI.NetworkInterfaceID = *output.NetworkInterfaces[0].NetworkInterfaceId
				}
			}
		}

		if len(desiredENI.NetworkInterfaceID) > 0 {
			deleteOnTermination = false
			networkInterfaceId = aws.String(desiredENI.NetworkInterfaceID)
		} else {
			deleteOnTermination = true

			if len(desiredENI.SubnetID) > 0 {
				subnetID = aws.String(desiredENI.SubnetID)
			} else {
				subnetID = aws.String(instance.config.Network.ENI[0].GetNextSubnetsID(nodeIndex))
			}

			if len(desiredENI.SecurityGroupID) > 0 {
				securityGroup = aws.String(desiredENI.SecurityGroupID)
			} else {
				securityGroup = aws.String(instance.config.Network.ENI[0].SecurityGroupID)
			}
		}

		return []*ec2.InstanceNetworkInterfaceSpecification{
			{
				AssociatePublicIpAddress: aws.Bool(desiredENI.PublicIP),
				DeleteOnTermination:      aws.Bool(deleteOnTermination),
				Description:              aws.String(instance.InstanceName),
				DeviceIndex:              aws.Int64(0),
				SubnetId:                 subnetID,
				NetworkInterfaceId:       networkInterfaceId,
				PrivateIpAddress:         privateIPAddress,
				Groups: []*string{
					securityGroup,
				},
			},
		}, nil

	} else if len(instance.config.Network.ENI) > 0 {
		interfaces := make([]*ec2.InstanceNetworkInterfaceSpecification, len(instance.config.Network.ENI))

		for index, eni := range instance.config.Network.ENI {
			inf := &ec2.InstanceNetworkInterfaceSpecification{
				AssociatePublicIpAddress: aws.Bool(eni.PublicIP),
				DeleteOnTermination:      aws.Bool(true),
				Description:              aws.String(instance.InstanceName),
				DeviceIndex:              aws.Int64(int64(index)),
				SubnetId:                 aws.String(eni.GetNextSubnetsID(nodeIndex)),
				Groups: []*string{
					aws.String(eni.SecurityGroupID),
				},
			}
			interfaces[index] = inf
		}

		return interfaces, nil
	} else {
		return nil, fmt.Errorf("unable create worker node, any network interface defined")
	}
}

func (instance *Ec2Instance) buildBlockDeviceMappings(diskType string, diskSize int) ([]*ec2.BlockDeviceMapping, error) {
	if diskSize > 0 || len(diskType) > 0 {
		if diskSize == 0 {
			diskSize = 20
		}

		if len(diskType) == 0 {
			diskType = "gp2"
		}

		ebs := &ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/sda1"),
			Ebs: &ec2.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				VolumeType:          aws.String(diskType),
				VolumeSize:          aws.Int64(int64(diskSize)),
			},
		}

		return []*ec2.BlockDeviceMapping{
			ebs,
		}, nil
	}

	return nil, nil
}

func (instance *Ec2Instance) buildTagSpecifications(nodeIndex int, nodeGroup string) ([]*ec2.TagSpecification, error) {
	instanceTags := make([]*ec2.Tag, 0, len(instance.config.Tags)+3)

	instanceTags = append(instanceTags, &ec2.Tag{
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

	instanceTags = append(instanceTags, &ec2.Tag{
		Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", nodeGroup)),
		Value: aws.String("owned"),
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

	return []*ec2.TagSpecification{
		{
			ResourceType: aws.String(ec2.ResourceTypeInstance),
			Tags:         instanceTags,
		},
	}, nil

}

// Create will create a named VM not powered
// memory and disk are in megabytes
func (instance *Ec2Instance) Create(nodeIndex int, nodeGroup, instanceType string, userData *string, diskType string, diskSize int, desiredENI *UserDefinedNetworkInterface) error {
	var err error
	var result *ec2.Reservation

	glog.Debugf("Create: instance name %s in node group %s", instance.InstanceName, nodeGroup)

	// Check if instance is not already created
	if _, err = GetEc2Instance(instance.config, instance.InstanceName); err == nil {
		glog.Debugf("Create: instance name %s already exists", instance.InstanceName)

		return fmt.Errorf(constantes.ErrCantCreateVMAlreadyExist, instance.InstanceName)
	}

	ctx := instance.NewContext()
	defer ctx.Cancel()

	input := &ec2.RunInstancesInput{
		InstanceType:                      aws.String(instanceType),
		ImageId:                           aws.String(instance.config.ImageID),
		KeyName:                           aws.String(instance.config.KeyName),
		InstanceInitiatedShutdownBehavior: aws.String(ec2.ShutdownBehaviorStop),
		MaxCount:                          aws.Int64(1),
		MinCount:                          aws.Int64(1),
		UserData:                          userData,
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Arn: &instance.config.IamRole,
		},
	}

	// Add tags
	if input.TagSpecifications, err = instance.buildTagSpecifications(nodeIndex, nodeGroup); err != nil {
		return err
	}

	// Add ENI
	if input.NetworkInterfaces, err = instance.buildNetworkInterfaces(nodeIndex, desiredENI); err != nil {
		return err
	}

	// Add Block device
	if input.BlockDeviceMappings, err = instance.buildBlockDeviceMappings(diskType, diskSize); err != nil {
		return err
	}

	if result, err = instance.client.RunInstancesWithContext(ctx, input); err != nil {
		return err
	}

	instance.Region = aws.String(instance.config.Region)
	instance.Zone = result.Instances[0].Placement.AvailabilityZone
	instance.InstanceID = result.Instances[0].InstanceId

	return nil
}

func (instance *Ec2Instance) delete(wait bool) error {
	ctx := instance.NewContext()
	defer ctx.Cancel()

	glog.Debugf("Delete: instance %s id (%s)", instance.InstanceName, instance.getInstanceID())

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{
			instance.InstanceID,
		},
	}

	if _, err := instance.client.TerminateInstancesWithContext(ctx, input); err != nil {
		return err
	}

	if wait {
		return instance.client.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
			InstanceIds: []*string{
				instance.InstanceID,
			},
		})

	}

	return nil
}

// Delete a VM by name and don't wait for terminated status
func (instance *Ec2Instance) Delete() error {
	return instance.delete(false)
}

// Terminate a VM by name and wait until status is terminated
func (instance *Ec2Instance) Terminate() error {
	return instance.delete(true)
}

// PowerOn power on a VM by name
func (instance *Ec2Instance) PowerOn() error {
	var err error

	ctx := instance.NewContext()
	input := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			instance.InstanceID,
		},
	}

	defer ctx.Cancel()

	glog.Debugf("PowerOn: instance %s id (%s)", instance.InstanceName, instance.getInstanceID())

	if _, err = instance.client.StartInstancesWithContext(ctx, input); err == nil {
		// Wait start is effective
		input := &ec2.DescribeInstancesInput{
			InstanceIds: []*string{
				instance.InstanceID,
			},
		}

		if err = instance.client.WaitUntilInstanceRunning(input); err == nil {
			if ec2Instance, err := instance.getEc2Instance(); err == nil {
				if ec2Instance.PublicIpAddress != nil {
					instance.AddressIP = ec2Instance.PublicIpAddress
				} else {
					instance.AddressIP = ec2Instance.PrivateIpAddress
				}
			}
		}

	} else {
		glog.Debugf("powerOn: instance %s id (%s), got error %v", instance.InstanceName, instance.getInstanceID(), err)
	}

	return err
}

func (instance *Ec2Instance) powerOff(force bool) error {
	var err error

	ctx := instance.NewContext()
	input := &ec2.StopInstancesInput{
		Force: &force,
		InstanceIds: []*string{
			instance.InstanceID,
		},
	}

	defer ctx.Cancel()

	glog.Debugf("powerOff: instance %s id (%s)", instance.InstanceName, instance.getInstanceID())

	if _, err = instance.client.StopInstancesWithContext(ctx, input); err == nil {
		input := &ec2.DescribeInstancesInput{
			InstanceIds: []*string{
				instance.InstanceID,
			},
		}

		err = instance.client.WaitUntilInstanceStopped(input)
	} else {
		glog.Debugf("powerOff: instance %s id (%s), got error %v", instance.InstanceName, instance.getInstanceID(), err)
	}

	return err
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

	glog.Debugf("Status: instance %s id (%s)", instance.InstanceName, instance.getInstanceID())

	if ec2Instance, err := instance.getEc2Instance(); err != nil {
		return nil, err
	} else {

		var address *string

		code := ec2Instance.State.Code

		if code == nil || *code == 48 {
			glog.Debugf("Status: instance %s id (%s) is terminated", instance.InstanceName, instance.getInstanceID())

			return nil, fmt.Errorf("EC2 Instance %s is terminated", instance.InstanceName)
		} else if *code == 16 || *code == 0 {
			if ec2Instance.PublicIpAddress != nil {
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

func (instance *Ec2Instance) getRegisteredRecordSetAddress(conf *Configuration, name string) (*string, error) {
	if session, e := newSessionWithOptions(conf.GetRoute53AccessKey(), conf.GetRoute53SecretKey(), conf.GetRoute53AccessToken(), conf.GetFileName(), conf.GetRoute53Profile(), conf.GetRoute53Region()); e == nil {
		svc := route53.New(session)

		input := &route53.ListResourceRecordSetsInput{
			HostedZoneId:    aws.String(conf.Network.ZoneID),
			MaxItems:        aws.String("1"),
			StartRecordName: aws.String(name),
			StartRecordType: aws.String("A"),
		}

		if output, err := svc.ListResourceRecordSets(input); err == nil {
			if len(output.ResourceRecordSets) > 0 && len(output.ResourceRecordSets[0].ResourceRecords) > 0 {
				return output.ResourceRecordSets[0].ResourceRecords[0].Value, nil
			} else {
				return nil, fmt.Errorf("route53 entry:%s not found", name)
			}
		} else {
			return nil, err
		}
	} else {
		return nil, e
	}
}

func (instance *Ec2Instance) changeResourceRecordSetsInput(conf *Configuration, cmd, name, address string, wait bool) error {
	var svc *route53.Route53

	if session, e := newSessionWithOptions(conf.GetRoute53AccessKey(), conf.GetRoute53SecretKey(), conf.GetRoute53AccessToken(), conf.GetFileName(), conf.GetRoute53Profile(), conf.GetRoute53Region()); e != nil {
		return e
	} else {
		svc = route53.New(session)

		input := &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: aws.String(conf.Network.ZoneID),
			ChangeBatch: &route53.ChangeBatch{
				Comment: aws.String("Kubernetes worker node"),
				Changes: []*route53.Change{
					{
						Action: aws.String(cmd),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String(name),
							TTL:  aws.Int64(60),
							Type: aws.String("A"),
							ResourceRecords: []*route53.ResourceRecord{
								{
									Value: aws.String(address),
								},
							},
						},
					},
				},
			},
		}

		result, err := svc.ChangeResourceRecordSets(input)

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case route53.ErrCodeNoSuchHostedZone:
					return fmt.Errorf("%s, %v", route53.ErrCodeNoSuchHostedZone, aerr.Error())
				case route53.ErrCodeNoSuchHealthCheck:
					return fmt.Errorf("%s, %v", route53.ErrCodeNoSuchHealthCheck, aerr.Error())
				case route53.ErrCodeInvalidChangeBatch:
					return fmt.Errorf("%s, %v", route53.ErrCodeInvalidChangeBatch, aerr.Error())
				case route53.ErrCodeInvalidInput:
					return fmt.Errorf("%s, %v", route53.ErrCodeInvalidInput, aerr.Error())
				case route53.ErrCodePriorRequestNotComplete:
					return fmt.Errorf("%s, %v", route53.ErrCodePriorRequestNotComplete, aerr.Error())
				default:
					return aerr
				}
			} else {
				return err
			}
		}

		if wait {
			input := &route53.GetChangeInput{
				Id: result.ChangeInfo.Id,
			}

			return svc.WaitUntilResourceRecordSetsChanged(input)
		}

		return nil
	}
}

// RegisterDNS register EC2 instance in Route53
func (instance *Ec2Instance) RegisterDNS(conf *Configuration, name, address string, wait bool) error {
	return instance.changeResourceRecordSetsInput(conf, route53_UpsertCmd, name, address, wait)
}

// UnRegisterDNS unregister EC2 instance in Route53
func (instance *Ec2Instance) UnRegisterDNS(conf *Configuration, name string, wait bool) error {
	if address, err := instance.getRegisteredRecordSetAddress(conf, name); err == nil {
		return instance.changeResourceRecordSetsInput(conf, route53_DeleteCmd, name, *address, wait)
	}

	return nil
}
