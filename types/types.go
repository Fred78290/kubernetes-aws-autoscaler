package types

import (
	"os/user"
	"strings"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/golang/glog"
)

// ResourceLimiter define limit, not really used
type ResourceLimiter struct {
	MinLimits map[string]int64 `json:"min"`
	MaxLimits map[string]int64 `json:"max"`
}

// MachineCharacteristic defines VM kind
type MachineCharacteristic struct {
	Price  float64 `json:"price"`    // VM price in USD
	Memory int     `json:"memsize"`  // VM Memory size in megabytes
	Vcpu   int     `json:"vcpus"`    // VM number of cpus
	Disk   int     `json:"disksize"` // VM disk size in megabytes
}

// KubeJoinConfig give element to join kube master
type KubeJoinConfig struct {
	Address        string   `json:"address,omitempty"`
	Token          string   `json:"token,omitempty"`
	CACert         string   `json:"ca,omitempty"`
	ExtraArguments []string `json:"extras-args,omitempty"`
}

// AutoScalerServerOptionals declare wich features must be optional
type AutoScalerServerOptionals struct {
	Pricing                  bool `json:"pricing"`
	GetAvailableMachineTypes bool `json:"getAvailableMachineTypes"`
	NewNodeGroup             bool `json:"newNodeGroup"`
	TemplateNodeInfo         bool `json:"templateNodeInfo"`
	Create                   bool `json:"create"`
	Delete                   bool `json:"delete"`
}

// AutoScalerServerSSH contains ssh client infos
type AutoScalerServerSSH struct {
	UserName string `json:"user"`
	Password string `json:"password"`
	AuthKeys string `json:"ssh-private-key"`
}

// GetUserName returns user name from config or the real current username is empty or equal to ~
func (ssh *AutoScalerServerSSH) GetUserName() string {
	if ssh.UserName == "" || ssh.UserName == "~" {
		u, err := user.Current()

		if err != nil {
			glog.Fatalf("Can't find current user! - %v", err)
		}

		return u.Username
	}

	return ssh.UserName
}

// GetAuthKeys returns the path to key file, subsistute ~
func (ssh *AutoScalerServerSSH) GetAuthKeys() string {
	if strings.Index(ssh.AuthKeys, "~") == 0 {
		u, err := user.Current()

		if err != nil {
			glog.Fatalf("Can't find current user! - %v", err)
		}

		return strings.Replace(ssh.AuthKeys, "~", u.HomeDir, 1)
	}

	return ssh.AuthKeys
}

// AutoScalerServerRsync declare an rsync operation
type AutoScalerServerRsync struct {
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	Excludes    []string `json:"excludes"`
}

// AutoScalerServerSyncFolders declare how to sync file between host and guest
type AutoScalerServerSyncFolders struct {
	RsyncOptions []string                `json:"options"`
	RsyncUser    string                  `json:"user"`
	RsyncSSHKey  string                  `json:"ssh-key"`
	Folders      []AutoScalerServerRsync `json:"folders"`
}

// AutoScalerServerConfig is contains configuration
type AutoScalerServerConfig struct {
	Network            string                            `default:"tcp" json:"network"`         // Mandatory, Network to listen (see grpc doc) to listen
	Listen             string                            `default:"0.0.0.0:5200" json:"listen"` // Mandatory, Address to listen
	ProviderID         string                            `json:"secret"`                        // Mandatory, secret Identifier, client must match this
	MinNode            int                               `json:"minNode"`                       // Mandatory, Min AutoScaler VM
	MaxNode            int                               `json:"maxNode"`                       // Mandatory, Max AutoScaler VM
	MaxPods            int                               `json:"maxPods"`                       // Mandatory, Max kubelet pods
	NodePrice          float64                           `json:"nodePrice"`                     // Optional, The VM price
	PodPrice           float64                           `json:"podPrice"`                      // Optional, The pod price
	KubeConfig         string                            `json:"-"`
	KubeAdm            KubeJoinConfig                    `json:"kubeadm"`
	DefaultMachineType string                            `default:"{\"standard\": {}}" json:"default-machine"`
	Machines           map[string]*MachineCharacteristic `default:"{\"standard\": {}}" json:"machines"` // Mandatory, Available machines
	CloudInit          interface{}                       `json:"cloud-init"`                            // Optional, The cloud init conf file
	SyncFolders        *AutoScalerServerSyncFolders      `json:"sync-folder"`                           // Optional, do rsync between host and guest
	Optionals          *AutoScalerServerOptionals        `json:"optionals"`
	SSH                *AutoScalerServerSSH              `json:"ssh-infos"`
	AwsInfos           map[string]*aws.Configuration     `json:"aws"`
}

// GetAwsConfiguration returns the aws named conf or default
func (conf *AutoScalerServerConfig) GetAwsConfiguration(name string) *aws.Configuration {
	var aws *aws.Configuration

	if aws = conf.AwsInfos[name]; aws == nil {
		aws = conf.AwsInfos["default"]
	}

	if aws == nil {
		glog.Fatalf("Unable to find aws config for name:%s", name)
	}

	return aws
}
