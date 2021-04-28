package client

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fred78290/kubernetes-aws-autoscaler/constantes"
	"github.com/Fred78290/kubernetes-aws-autoscaler/context"
	"github.com/Fred78290/kubernetes-aws-autoscaler/types"
	"github.com/Fred78290/kubernetes-aws-autoscaler/utils"

	"github.com/linki/instrumented_http"
	glog "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Default pod eviction settings.
const (
	conditionDrainedScheduled = "DrainScheduled"
)

// SingletonClientGenerator provides clients
type SingletonClientGenerator struct {
	KubeConfig      string
	APIServerURL    string
	RequestTimeout  time.Duration
	DeletionTimeout time.Duration
	MaxGracePeriod  time.Duration
	kubeClient      kubernetes.Interface
	kubeOnce        sync.Once
}

// getRestConfig returns the rest clients config to get automatically
// data if you run inside a cluster or by passing flags.
func getRestConfig(kubeConfig, apiServerURL string) (*rest.Config, error) {
	if kubeConfig == "" {
		if _, err := os.Stat(clientcmd.RecommendedHomeFile); err == nil {
			kubeConfig = clientcmd.RecommendedHomeFile
		}
	}

	glog.Debugf("apiServerURL: %s", apiServerURL)
	glog.Debugf("kubeConfig: %s", kubeConfig)

	// evaluate whether to use kubeConfig-file or serviceaccount-token
	var (
		config *rest.Config
		err    error
	)
	if kubeConfig == "" {
		glog.Infof("Using inCluster-config based on serviceaccount-token")
		config, err = rest.InClusterConfig()
	} else {
		glog.Infof("Using kubeConfig")
		config, err = clientcmd.BuildConfigFromFlags(apiServerURL, kubeConfig)
	}
	if err != nil {
		return nil, err
	}

	return config, nil
}

// newKubeClient returns a new Kubernetes client object. It takes a Config and
// uses APIServerURL and KubeConfig attributes to connect to the cluster. If
// KubeConfig isn't provided it defaults to using the recommended default.
func newKubeClient(kubeConfig, apiServerURL string, requestTimeout time.Duration) (*kubernetes.Clientset, error) {
	glog.Infof("Instantiating new Kubernetes client")

	config, err := getRestConfig(kubeConfig, apiServerURL)
	if err != nil {
		return nil, err
	}

	config.Timeout = requestTimeout * time.Second

	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return instrumented_http.NewTransport(rt, &instrumented_http.Callbacks{
			PathProcessor: func(path string) string {
				parts := strings.Split(path, "/")
				return parts[len(parts)-1]
			},
		})
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	glog.Infof("Created Kubernetes client %s", config.Host)

	return client, nil
}

func (p *SingletonClientGenerator) newRequestContext() *context.Context {
	return context.NewContext(time.Duration(p.RequestTimeout.Seconds()))
}

// KubeClient generates a kube client if it was not created before
func (p *SingletonClientGenerator) KubeClient() (kubernetes.Interface, error) {
	var err error
	p.kubeOnce.Do(func() {
		p.kubeClient, err = newKubeClient(p.KubeConfig, p.APIServerURL, p.RequestTimeout)
	})
	return p.kubeClient, err
}

func (p *SingletonClientGenerator) WaitNodeToBeReady(nodeName string, timeToWaitInSeconds int) error {
	var nodeInfo *apiv1.Node
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	timeout := time.Duration(timeToWaitInSeconds) * time.Second

	glog.Infof("Wait kubernetes node %s to be ready", nodeName)

	if err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		nodeInfo, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})

		if err != nil {
			return false, err
		}

		for _, status := range nodeInfo.Status.Conditions {
			if status.Type == apiv1.NodeReady {
				if b, e := strconv.ParseBool(string(status.Status)); e == nil {
					if b {
						return true, nil
					}
				}
			}
		}

		glog.Debugf("The kubernetes node:%s is not ready", nodeName)

		return false, nil
	}); err == nil {
		glog.Infof("The kubernetes node %s is Ready", nodeName)
		return nil
	}

	return fmt.Errorf(constantes.ErrNodeIsNotReady, nodeName)
}

func (p *SingletonClientGenerator) awaitDeletion(pod apiv1.Pod, timeout time.Duration) error {
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	return wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		got, err := kubeclient.CoreV1().Pods(pod.GetNamespace()).Get(ctx, pod.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, fmt.Errorf(constantes.ErrUndefinedPod, pod.GetNamespace(), pod.GetName(), err)
		}
		if got.GetUID() != pod.GetUID() {
			return true, nil
		}
		return false, nil
	})
}

func (p *SingletonClientGenerator) evictPod(pod apiv1.Pod, abort <-chan struct{}, e chan<- error) {
	gracePeriod := int64(p.MaxGracePeriod.Seconds())

	if pod.Spec.TerminationGracePeriodSeconds != nil && *pod.Spec.TerminationGracePeriodSeconds < gracePeriod {
		gracePeriod = *pod.Spec.TerminationGracePeriodSeconds
	}

	kubeclient, err := p.KubeClient()

	if err != nil {
		e <- err
		return
	}

	ctx := context.NewContext(time.Duration(gracePeriod))
	defer ctx.Cancel()

	for {
		select {
		case <-abort:
			e <- fmt.Errorf(constantes.ErrPodEvictionAborted)
			return
		default:
			err := kubeclient.CoreV1().Pods(pod.GetNamespace()).Evict(ctx, &policy.Eviction{
				ObjectMeta:    metav1.ObjectMeta{Namespace: pod.GetNamespace(), Name: pod.GetName()},
				DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
			})
			switch {
			// The eviction API returns 429 Too Many Requests if a pod
			// cannot currently be evicted, for example due to a pod
			// disruption budget.
			case apierrors.IsTooManyRequests(err):
				time.Sleep(5 * time.Second)
			case apierrors.IsNotFound(err):
				e <- nil
				return
			case err != nil:
				e <- fmt.Errorf(constantes.ErrCannotEvictPod, pod.GetNamespace(), pod.GetName(), err)
				return
			default:
				if err = p.awaitDeletion(pod, p.DeletionTimeout); err != nil {
					e <- fmt.Errorf(constantes.ErrUnableToConfirmPodEviction, pod.GetNamespace(), pod.GetName(), err)
				} else {
					e <- nil
				}
				return
			}
		}
	}
}

// PodList return list of pods hosted on named node
func (p *SingletonClientGenerator) PodList(nodeName string, podFilter types.PodFilterFunc) ([]apiv1.Pod, error) {
	var pods *apiv1.PodList

	kubeclient, err := p.KubeClient()

	if err != nil {
		return nil, err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	if pods, err = kubeclient.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String(),
	}); err != nil {
		return nil, fmt.Errorf(constantes.ErrPodListReturnError, nodeName, err)
	}

	include := make([]apiv1.Pod, 0, len(pods.Items))

	for _, pod := range pods.Items {
		passes, err := podFilter(pod)
		if err != nil {
			return nil, fmt.Errorf("cannot filter pods, reason: %v", err)
		}
		if passes {
			include = append(include, pod)
		}
	}

	return include, nil
}

// NodeList return node list from cluster
func (p *SingletonClientGenerator) NodeList() (*apiv1.NodeList, error) {

	kubeclient, err := p.KubeClient()

	if err != nil {
		return nil, err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	return kubeclient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
}

func (p *SingletonClientGenerator) cordonOrUncordonNode(nodeName string, flag bool) error {
	var node *apiv1.Node
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	if node, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	if node.Spec.Unschedulable == flag {
		return nil
	}

	node.Spec.Unschedulable = flag

	_, err = kubeclient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})

	return err
}

func (p *SingletonClientGenerator) UncordonNode(nodeName string) error {
	return p.cordonOrUncordonNode(nodeName, false)
}

func (p *SingletonClientGenerator) CordonNode(nodeName string) error {
	return p.cordonOrUncordonNode(nodeName, true)
}

func (p *SingletonClientGenerator) SetProviderID(nodeName, providerID string) error {
	var node *apiv1.Node
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	if node, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	if node.Spec.ProviderID == providerID {
		return nil
	}

	node.Spec.ProviderID = providerID

	_, err = kubeclient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})

	return err
}

func (p *SingletonClientGenerator) MarkDrainNode(nodeName string) error {
	var node *apiv1.Node
	now := metav1.Time{Time: time.Now()}
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	if node, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	conditionStatus := apiv1.ConditionTrue

	// Create or update the condition associated to the monitor
	conditionUpdated := false

	for i, condition := range node.Status.Conditions {
		if string(condition.Type) == conditionDrainedScheduled {
			node.Status.Conditions[i].LastHeartbeatTime = now
			node.Status.Conditions[i].Message = "Drain activity scheduled " + now.Time.Format(time.RFC3339)
			node.Status.Conditions[i].Status = conditionStatus
			conditionUpdated = true
			break
		}
	}

	if !conditionUpdated { // There was no condition found, let's create one
		node.Status.Conditions = append(node.Status.Conditions,
			apiv1.NodeCondition{
				Type:               apiv1.NodeConditionType(conditionDrainedScheduled),
				Status:             conditionStatus,
				LastHeartbeatTime:  now,
				LastTransitionTime: now,
				Reason:             "Draino",
				Message:            "Drain activity scheduled " + now.Format(time.RFC3339),
			},
		)
	}

	if _, err = kubeclient.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

func (p *SingletonClientGenerator) DrainNode(nodeName string, ignoreDaemonSet, deleteLocalData bool) error {
	ctx := p.newRequestContext()
	defer ctx.Cancel()

	pf := []types.PodFilterFunc{utils.MirrorPodFilter}

	if ignoreDaemonSet {
		pf = append(pf, utils.NewDaemonSetPodFilter(ctx, p.kubeClient))
	}

	if !deleteLocalData {
		pf = append(pf, utils.LocalStoragePodFilter)
	}

	pods, err := p.PodList(nodeName, utils.NewPodFilters(pf...))
	if err != nil {
		return fmt.Errorf(constantes.ErrUnableToGetPodListOnNode, nodeName, err)
	}

	abort := make(chan struct{})
	errs := make(chan error, 1)

	defer close(abort)

	for _, pod := range pods {
		go p.evictPod(pod, abort, errs)
	}

	deadline := time.After(p.RequestTimeout)

	for range pods {
		select {
		case err := <-errs:
			if err != nil {
				return fmt.Errorf(constantes.ErrUnableEvictAllPodsOnNode, nodeName, err)
			}
		case <-deadline:
			return fmt.Errorf(constantes.ErrTimeoutWhenWaitingEvictions, nodeName)
		}
	}

	return nil
}

func (p *SingletonClientGenerator) DeleteNode(nodeName string) error {
	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	return kubeclient.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
}

// AnnoteNode set annotation on node
func (p *SingletonClientGenerator) AnnoteNode(nodeName string, annotations map[string]string) error {
	var nodeInfo *apiv1.Node

	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	if nodeInfo, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	if len(nodeInfo.Annotations) == 0 {
		nodeInfo.Annotations = annotations
	} else {
		for k, v := range annotations {
			nodeInfo.Annotations[k] = v
		}
	}

	_, err = kubeclient.CoreV1().Nodes().Update(ctx, nodeInfo, metav1.UpdateOptions{})

	return err
}

// AnnoteNode set annotation on node
func (p *SingletonClientGenerator) LabelNode(nodeName string, labels map[string]string) error {
	var nodeInfo *apiv1.Node

	kubeclient, err := p.KubeClient()

	if err != nil {
		return err
	}

	ctx := p.newRequestContext()
	defer ctx.Cancel()

	if nodeInfo, err = kubeclient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err != nil {
		return err
	}

	if len(nodeInfo.Labels) == 0 {
		nodeInfo.Labels = labels
	} else {
		for k, v := range labels {
			nodeInfo.Labels[k] = v
		}
	}

	_, err = kubeclient.CoreV1().Nodes().Update(ctx, nodeInfo, metav1.UpdateOptions{})

	return err
}

func NewClientGenerator(cfg *types.Config) *SingletonClientGenerator {
	return &SingletonClientGenerator{
		KubeConfig:      cfg.KubeConfig,
		APIServerURL:    cfg.APIServerURL,
		RequestTimeout:  cfg.RequestTimeout,
		DeletionTimeout: cfg.DeletionTimeout,
		MaxGracePeriod:  cfg.MaxGracePeriod,
	}
}
