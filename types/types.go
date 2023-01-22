package types

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/Fred78290/kubernetes-aws-autoscaler/aws"
	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	apigrpc "github.com/Fred78290/kubernetes-aws-autoscaler/grpc"
	"github.com/alecthomas/kingpin"
	glog "github.com/sirupsen/logrus"

	clientset "github.com/Fred78290/kubernetes-aws-autoscaler/pkg/generated/clientset/versioned"
	apiv1 "k8s.io/api/core/v1"
	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultMaxGracePeriod    time.Duration = 120 * time.Second
	DefaultMaxRequestTimeout time.Duration = 120 * time.Second
	DefaultMaxDeletionPeriod time.Duration = 300 * time.Second
	DefaultNodeReadyTimeout  time.Duration = 300 * time.Second
)

const (
	ManagedNodeMinMemory   = 2 * 1024
	ManagedNodeMaxMemory   = 128 * 1024
	ManagedNodeMinCores    = 2
	ManagedNodeMaxCores    = 32
	ManagedNodeMinDiskSize = 10
	ManagedNodeMaxDiskSize = 1024 * 1024
)

// KubernetesLabel labels
type KubernetesLabel map[string]string

type Config struct {
	APIServerURL             string
	KubeConfig               string
	ExtDestinationEtcdSslDir string
	ExtSourceEtcdSslDir      string
	KubernetesPKISourceDir   string
	KubernetesPKIDestDir     string
	UseExternalEtdc          bool
	UseVanillaGrpcProvider   bool
	RequestTimeout           time.Duration
	DeletionTimeout          time.Duration
	MaxGracePeriod           time.Duration
	NodeReadyTimeout         time.Duration
	Config                   string
	SaveLocation             string
	DisplayVersion           bool
	LogFormat                string
	LogLevel                 string
	MinCpus                  int64
	MinMemory                int64
	MaxCpus                  int64
	MaxMemory                int64
	ManagedNodeMinDiskSize   int64
	ManagedNodeMaxDiskSize   int64
	ManagedNodeDiskType      string
}

func (c *Config) GetResourceLimiter() *ResourceLimiter {
	return &ResourceLimiter{
		MinLimits: map[string]int64{
			constantes.ResourceNameCores:  c.MinCpus,
			constantes.ResourceNameMemory: c.MinMemory * 1024 * 1024,
		},
		MaxLimits: map[string]int64{
			constantes.ResourceNameCores:  c.MaxCpus,
			constantes.ResourceNameMemory: c.MaxMemory * 1024 * 1024,
		},
	}
}

func (c *Config) GetManagedNodeResourceLimiter() *ResourceLimiter {
	return &ResourceLimiter{
		MinLimits: map[string]int64{
			constantes.ResourceNameManagedNodeDisk: c.ManagedNodeMinDiskSize,
		},
		MaxLimits: map[string]int64{
			constantes.ResourceNameManagedNodeDisk: c.ManagedNodeMaxDiskSize,
		},
	}
}

// A PodFilterFunc returns true if the supplied pod passes the filter.
type PodFilterFunc func(p apiv1.Pod) (bool, error)

// ClientGenerator provides clients
type ClientGenerator interface {
	KubeClient() (kubernetes.Interface, error)
	NodeManagerClient() (clientset.Interface, error)
	ApiExtentionClient() (apiextension.Interface, error)

	PodList(nodeName string, podFilter PodFilterFunc) ([]apiv1.Pod, error)
	NodeList() (*apiv1.NodeList, error)
	GetNode(nodeName string) (*apiv1.Node, error)
	SetProviderID(nodeName, providerID string) error
	UncordonNode(nodeName string) error
	CordonNode(nodeName string) error
	MarkDrainNode(nodeName string) error
	DrainNode(nodeName string, ignoreDaemonSet, deleteLocalData bool) error
	DeleteNode(nodeName string) error
	AnnoteNode(nodeName string, annotations map[string]string) error
	LabelNode(nodeName string, labels map[string]string) error
	TaintNode(nodeName string, taints ...apiv1.Taint) error
	WaitNodeToBeReady(nodeName string) error
}

// ResourceLimiter define limit, not really used
type ResourceLimiter struct {
	MinLimits map[string]int64 `json:"min"`
	MaxLimits map[string]int64 `json:"max"`
}

// MachineCharacteristic defines VM kind
type MachineCharacteristic struct {
	Price    float64 `json:"price"`                  // VM price in USD
	Memory   int     `json:"memsize"`                // VM Memory size in megabytes
	Vcpu     int     `json:"vcpus"`                  // VM number of cpus
	DiskType string  `default:"gp2" json:"diskType"` // VM disk size type gp2, gp3.....
	DiskSize int     `json:"diskSize"`               // VM disk size in megabytes
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
	TestMode bool   `json:"-"`
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

type NodeGroupAutoscalingOptions struct {
	// ScaleDownUtilizationThreshold sets threshold for nodes to be considered for scale down
	// if cpu or memory utilization is over threshold.
	ScaleDownUtilizationThreshold float64 `json:"scaleDownUtilizationThreshold,omitempty"`

	// ScaleDownGpuUtilizationThreshold sets threshold for gpu nodes to be
	// considered for scale down if gpu utilization is over threshold.
	ScaleDownGpuUtilizationThreshold float64 `json:"scaleDownGpuUtilizationThreshold,omitempty"`

	// ScaleDownUnneededTime sets the duration CA expects a node to be
	// unneeded/eligible for removal before scaling down the node.
	ScaleDownUnneededTime time.Duration `json:"scaleDownUnneededTime,omitempty"`

	// ScaleDownUnreadyTime represents how long an unready node should be
	// unneeded before it is eligible for scale down.
	ScaleDownUnreadyTime time.Duration `json:"scaleDownUnreadyTime,omitempty"`
}

// AutoScalerServerConfig is contains configuration
type AutoScalerServerConfig struct {
	UseExternalEtdc            *bool                             `json:"use-external-etcd"`
	UseVanillaGrpcProvider     *bool                             `json:"use-vanilla-grpc"`
	ExtDestinationEtcdSslDir   string                            `default:"/etc/etcd/ssl" json:"dst-etcd-ssl-dir"`
	ExtSourceEtcdSslDir        string                            `default:"/etc/etcd/ssl" json:"src-etcd-ssl-dir"`
	KubernetesPKISourceDir     string                            `default:"/etc/kubernetes/pki" json:"kubernetes-pki-srcdir"`
	KubernetesPKIDestDir       string                            `default:"/etc/kubernetes/pki" json:"kubernetes-pki-dstdir"`
	Network                    string                            `default:"tcp" json:"network"`                     // Mandatory, Network to listen (see grpc doc) to listen
	Listen                     string                            `default:"0.0.0.0:5200" json:"listen"`             // Mandatory, Address to listen
	CertPrivateKey             string                            `json:"cert-private-key,omitempty"`                // Optional to secure grcp channel
	CertPublicKey              string                            `json:"cert-public-key,omitempty"`                 // Optional to secure grcp channel
	CertCA                     string                            `json:"cert-ca,omitempty"`                         // Optional to secure grcp channel
	ServiceIdentifier          string                            `json:"secret"`                                    // Mandatory, secret Identifier, client must match this
	MinNode                    int                               `json:"minNode"`                                   // Mandatory, Min AutoScaler VM
	MaxNode                    int                               `json:"maxNode"`                                   // Mandatory, Max AutoScaler VM
	MaxPods                    int                               `json:"maxPods"`                                   // Mandatory, Max kubelet pods
	MaxCreatedNodePerCycle     int                               `json:"maxNode-per-cycle" default:"2"`             // Optional, the max number VM to create in //
	ProvisionnedNodeNamePrefix string                            `default:"autoscaled" json:"node-name-prefix"`     // Optional, the created node name prefix
	ManagedNodeNamePrefix      string                            `default:"worker" json:"managed-name-prefix"`      // Optional, the created node name prefix
	ControlPlaneNamePrefix     string                            `default:"master" json:"controlplane-name-prefix"` // Optional, the created node name prefix
	NodePrice                  float64                           `json:"nodePrice"`                                 // Optional, The VM price
	PodPrice                   float64                           `json:"podPrice"`                                  // Optional, The pod price
	KubeAdm                    KubeJoinConfig                    `json:"kubeadm"`
	DefaultMachineType         string                            `default:"standard" json:"default-machine"`
	NodeLabels                 KubernetesLabel                   `json:"nodeLabels"`
	Machines                   map[string]*MachineCharacteristic `default:"{\"standard\": {}}" json:"machines"` // Mandatory, Available machines
	Optionals                  *AutoScalerServerOptionals        `json:"optionals"`
	ManagedNodeResourceLimiter *ResourceLimiter                  `json:"managednodes-limits"`
	SSH                        *AutoScalerServerSSH              `json:"ssh-infos"`
	AutoScalingOptions         *NodeGroupAutoscalingOptions      `json:"autoscaling-options,omitempty"`
	CloudProvider              string                            `json:"cloud-provider"`
	AwsInfos                   map[string]*aws.Configuration     `json:"aws"`
}

func (limits *ResourceLimiter) MergeRequestResourceLimiter(limiter *apigrpc.ResourceLimiter) {
	if limits.MaxLimits == nil {
		limits.MaxLimits = limiter.MaxLimits
	} else {
		for k, v := range limiter.MaxLimits {
			limits.MaxLimits[k] = v
		}
	}

	if limits.MinLimits == nil {
		limits.MinLimits = limiter.MinLimits
	} else {
		for k, v := range limiter.MinLimits {
			limits.MinLimits[k] = v
		}
	}
}

func (limits *ResourceLimiter) SetMaxValue64(key string, value int64) {
	if limits.MaxLimits == nil {
		limits.MaxLimits = make(map[string]int64)
	}

	limits.MaxLimits[key] = value
}

func (limits *ResourceLimiter) SetMinValue64(key string, value int64) {
	if limits.MinLimits == nil {
		limits.MinLimits = make(map[string]int64)
	}

	limits.MinLimits[key] = value
}

func (limits *ResourceLimiter) GetMaxValue64(key string, defaultValue int64) int64 {
	if limits.MaxLimits != nil {
		if value, found := limits.MaxLimits[key]; found {
			return value
		}
	}
	return defaultValue
}

func (limits *ResourceLimiter) GetMinValue64(key string, defaultValue int64) int64 {
	if limits.MinLimits != nil {
		if value, found := limits.MinLimits[key]; found {
			return value
		}
	}
	return defaultValue
}

func (limits *ResourceLimiter) SetMaxValue(key string, value int) {
	if limits.MaxLimits == nil {
		limits.MaxLimits = make(map[string]int64)
	}

	limits.MaxLimits[key] = int64(value)
}

func (limits *ResourceLimiter) SetMinValue(key string, value int) {
	if limits.MinLimits == nil {
		limits.MinLimits = make(map[string]int64)
	}

	limits.MinLimits[key] = int64(value)
}

func (limits *ResourceLimiter) GetMaxValue(key string, defaultValue int) int {
	if limits.MaxLimits != nil {
		if value, found := limits.MaxLimits[key]; found {
			return int(value)
		}
	}
	return defaultValue
}

func (limits *ResourceLimiter) GetMinValue(key string, defaultValue int) int {
	if limits.MinLimits != nil {
		if value, found := limits.MinLimits[key]; found {
			return int(value)
		}
	}
	return defaultValue
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

// NewConfig returns new Config object
func NewConfig() *Config {
	return &Config{
		APIServerURL:             "",
		KubeConfig:               "",
		UseExternalEtdc:          false,
		UseVanillaGrpcProvider:   false,
		ExtDestinationEtcdSslDir: "/etc/etcd/ssl",
		ExtSourceEtcdSslDir:      "/etc/etcd/ssl",
		KubernetesPKISourceDir:   "/etc/kubernetes/pki",
		KubernetesPKIDestDir:     "/etc/kubernetes/pki",
		RequestTimeout:           DefaultMaxRequestTimeout,
		DeletionTimeout:          DefaultMaxDeletionPeriod,
		MaxGracePeriod:           DefaultMaxGracePeriod,
		NodeReadyTimeout:         DefaultNodeReadyTimeout,
		DisplayVersion:           false,
		Config:                   "/etc/cluster/aws-cluster-autoscaler.json",
		MinCpus:                  2,
		MinMemory:                1024,
		MaxCpus:                  24,
		MaxMemory:                1024 * 24,
		ManagedNodeMinDiskSize:   ManagedNodeMinDiskSize,
		ManagedNodeMaxDiskSize:   ManagedNodeMaxDiskSize,
		ManagedNodeDiskType:      "gp2",
		LogFormat:                "text",
		LogLevel:                 glog.InfoLevel.String(),
	}
}

// allLogLevelsAsStrings returns all logrus levels as a list of strings
func allLogLevelsAsStrings() []string {
	var levels []string
	for _, level := range glog.AllLevels {
		levels = append(levels, level.String())
	}
	return levels
}

func (cfg *Config) ParseFlags(args []string, version string) error {
	app := kingpin.New("aws-autoscaler", "Kubernetes AWS autoscaler create EC2 instances at demand for autoscaling.\n\nNote that all flags may be replaced with env vars - `--flag` -> `VMWARE_AUTOSCALER_FLAG=1` or `--flag value` -> `VMWARE_AUTOSCALER_FLAG=value`")

	//	app.Version(version)
	app.HelpFlag.Short('h')
	app.DefaultEnvars()

	app.Flag("log-format", "The format in which log messages are printed (default: text, options: text, json)").Default(cfg.LogFormat).EnumVar(&cfg.LogFormat, "text", "json")
	app.Flag("log-level", "Set the level of logging. (default: info, options: panic, debug, info, warning, error, fatal").Default(cfg.LogLevel).EnumVar(&cfg.LogLevel, allLogLevelsAsStrings()...)

	app.Flag("use-vanilla-grpc", "Tell we use vanilla autoscaler externalgrpc cloudprovider").Default("false").BoolVar(&cfg.UseVanillaGrpcProvider)

	// External Etcd
	app.Flag("use-external-etcd", "Tell we use an external etcd service (overriden by config file if defined)").Default("false").BoolVar(&cfg.UseExternalEtdc)
	app.Flag("src-etcd-ssl-dir", "Locate the source etcd ssl files (overriden by config file if defined)").Default(cfg.ExtSourceEtcdSslDir).StringVar(&cfg.ExtSourceEtcdSslDir)
	app.Flag("dst-etcd-ssl-dir", "Locate the destination etcd ssl files (overriden by config file if defined)").Default(cfg.ExtDestinationEtcdSslDir).StringVar(&cfg.ExtDestinationEtcdSslDir)

	app.Flag("kubernetes-pki-srcdir", "Locate the source kubernetes pki files (overriden by config file if defined)").Default(cfg.KubernetesPKISourceDir).StringVar(&cfg.KubernetesPKISourceDir)
	app.Flag("kubernetes-pki-dstdir", "Locate the destination kubernetes pki files (overriden by config file if defined)").Default(cfg.KubernetesPKIDestDir).StringVar(&cfg.KubernetesPKIDestDir)

	// Flags related to Kubernetes
	app.Flag("server", "The Kubernetes API server to connect to (default: auto-detect)").Default(cfg.APIServerURL).StringVar(&cfg.APIServerURL)
	app.Flag("kubeconfig", "Retrieve target cluster configuration from a Kubernetes configuration file (default: auto-detect)").Default(cfg.KubeConfig).StringVar(&cfg.KubeConfig)
	app.Flag("request-timeout", "Request timeout when calling Kubernetes APIs. 0s means no timeout").Default(DefaultMaxRequestTimeout.String()).DurationVar(&cfg.RequestTimeout)
	app.Flag("deletion-timeout", "Deletion timeout when delete node. 0s means no timeout").Default(DefaultMaxDeletionPeriod.String()).DurationVar(&cfg.DeletionTimeout)
	app.Flag("max-grace-period", "Maximum time evicted pods will be given to terminate gracefully.").Default(DefaultMaxGracePeriod.String()).DurationVar(&cfg.MaxGracePeriod)

	app.Flag("min-cpus", "Limits: minimum cpu (default: 1)").Default(strconv.FormatInt(cfg.MinCpus, 10)).Int64Var(&cfg.MinCpus)
	app.Flag("max-cpus", "Limits: max cpu (default: 24)").Default(strconv.FormatInt(cfg.MaxCpus, 10)).Int64Var(&cfg.MaxCpus)
	app.Flag("min-memory", "Limits: minimum memory in MB (default: 1G)").Default(strconv.FormatInt(cfg.MinMemory, 10)).Int64Var(&cfg.MinMemory)
	app.Flag("max-memory", "Limits: max memory in MB (default: 24G)").Default(strconv.FormatInt(cfg.MaxMemory, 10)).Int64Var(&cfg.MaxMemory)

	app.Flag("min-managednode-disksize", "Managed node: minimum disk size in MB (default: 10MB)").Default(strconv.FormatInt(cfg.ManagedNodeMinDiskSize, 10)).Int64Var(&cfg.ManagedNodeMinDiskSize)
	app.Flag("max-managednode-disksize", "Managed node: max disk size in MB (default: 1T)").Default(strconv.FormatInt(cfg.ManagedNodeMaxDiskSize, 10)).Int64Var(&cfg.ManagedNodeMaxDiskSize)
	app.Flag("managednode-disktype", "Managed node: define the disk type (default: gp2)").Default(cfg.ManagedNodeDiskType).StringVar(&cfg.ManagedNodeDiskType)

	app.Flag("version", "Display version and exit").BoolVar(&cfg.DisplayVersion)

	app.Flag("config", "The config for the server").Default(cfg.Config).StringVar(&cfg.Config)
	app.Flag("save", "The file to persists the server").Default(cfg.SaveLocation).StringVar(&cfg.SaveLocation)

	_, err := app.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

func (cfg *Config) String() string {
	return fmt.Sprintf("APIServerURL:%s KubeConfig:%s RequestTimeout:%s Config:%s SaveLocation:%s DisplayVersion:%s", cfg.APIServerURL, cfg.KubeConfig, cfg.RequestTimeout, cfg.Config, cfg.SaveLocation, strconv.FormatBool(cfg.DisplayVersion))
}
