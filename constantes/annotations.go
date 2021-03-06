package constantes

const (
	// ResourceNameCores is string name for cores. It's used by ResourceLimiter.
	ResourceNameCores = "cpu"
	// ResourceNameMemory is string name for memory. It's used by ResourceLimiter.
	// Memory should always be provided in bytes.
	ResourceNameMemory = "memory"
)

const (
	// NodeLabelWorkerRole k8s annotation
	NodeLabelWorkerRole = "node-role.kubernetes.io/worker"

	// NodeLabelGroupName k8s annotation
	NodeLabelGroupName = "cluster.autoscaler.nodegroup/name"

	// AnnotationInstanceID k8s annotation
	AnnotationInstanceID = "cluster.autoscaler.nodegroup/instance-id"

	// AnnotationInstanceName k8s annotation
	AnnotationInstanceName = "cluster.autoscaler.nodegroup/instance-name"

	// AnnotationNodeIndex k8s annotation
	AnnotationNodeIndex = "cluster.autoscaler.nodegroup/node-index"

	// AnnotationNodeAutoProvisionned k8s annotation
	AnnotationNodeAutoProvisionned = "cluster.autoscaler.nodegroup/autoprovision"

	// AnnotationScaleDownDisabled k8s annotation
	AnnotationScaleDownDisabled = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
)
