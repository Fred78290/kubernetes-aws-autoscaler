package constantes

const (
	// CloudProviderError is an error related to underlying infrastructure
	CloudProviderError = "CloudProviderError"

	// APICallError is an error related to communication with k8s API server
	APICallError = "ApiCallError"

	// InternalError is an error inside Cluster Autoscaler
	InternalError = "InternalError"

	// TransientError is an error that causes us to skip a single loop, but
	// does not require any additional action.
	TransientError = "TransientError"
)

const (
	// ProviderName string
	ProviderName = "grpc"

	// ErrMismatchingProvider error msg
	ErrMismatchingProvider = "secret doesn't match with target server"

	// ErrNodeGroupNotFound error msg
	ErrNodeGroupNotFound = "node group %s not found"

	// ErrNodeGroupForNodeNotFound error msg
	ErrNodeGroupForNodeNotFound = "nodeGroup %s not found for Node %s"

	// ErrNodeNotFoundInNodeGroup error msg
	ErrNodeNotFoundInNodeGroup = "the node %s not found in node group %s"

	// ErrMachineTypeNotFound error msg
	ErrMachineTypeNotFound = "machine type %s not found"

	// ErrNodeGroupAlreadyExists error msg
	ErrNodeGroupAlreadyExists = "can't create node group: %s, already exists"

	// ErrUnableToCreateNodeGroup error msg
	ErrUnableToCreateNodeGroup = "can't create node group: %s, reason: %v"

	// ErrUnableToDeleteNodeGroup error msg
	ErrUnableToDeleteNodeGroup = "can't delete node group: %s, reason: %v"

	// ErrCantDecodeNodeIDWithReason error msg
	ErrCantDecodeNodeIDWithReason = "node providerID %s not conform, reason: %v"

	// ErrCantDecodeNodeID error msg
	ErrCantDecodeNodeID = "node providerID %s not conform"

	// ErrCantUnmarshallNodeWithReason error msg
	ErrCantUnmarshallNodeWithReason = "can't unmarshall node definition:%s, reason: %v"

	// ErrCantUnmarshallNode error msg
	ErrCantUnmarshallNode = "can't unmarshall node definition[%d] in group %s"

	// ErrUnableToDeleteNode error msg
	ErrUnableToDeleteNode = "can't delete node: %s, because not owned by node group: %s"

	// ErrMinSizeReached error msg
	ErrMinSizeReached = "min size reached for group: %s, nodes will not be deleted"

	// ErrIncreaseSizeMustBePositive error msg
	ErrIncreaseSizeMustBePositive = "size increase must be positive"

	// ErrIncreaseSizeTooLarge error msg
	ErrIncreaseSizeTooLarge = "size increase too large, desired: %d max: %d"

	// ErrDecreaseSizeMustBeNegative error msg
	ErrDecreaseSizeMustBeNegative = "size decrease must be negative"

	// ErrDecreaseSizeAttemptDeleteNodes error msg
	ErrDecreaseSizeAttemptDeleteNodes = "attempt to delete existing nodes, targetSize: %d delta: %d existingNodes: %d"

	// ErrUnableToLaunchVM error msg
	ErrUnableToLaunchVM = "unable to launch the VM owned by node: %s, reason: %v"

	// ErrUnableToDeleteVM error msg
	ErrUnableToDeleteVM = "unable to delete the VM owned by node: %s, reason: %v"

	// ErrWrongSchemeInProviderID error msg
	ErrWrongSchemeInProviderID = "wrong scheme in providerID %s. expect %s, got: %s"

	// ErrWrongPathInProviderID error msg
	ErrWrongPathInProviderID = "wrong path in providerID: %s. expect object, got: %s"

	// ErrVMAlreadyCreated error msg
	ErrVMAlreadyCreated = "unable to launch VM, %s is already created"

	// ErrUnableToMountPath error msg
	ErrUnableToMountPath = "unable to mount host path:%s into guest:%s for node:%s, reason: %v"

	// ErrTempFile error msg
	ErrTempFile = "can't create temp file, reason: %v"

	// ErrGetVMInfoFailed error msg
	ErrGetVMInfoFailed = "can't get the VM info from AutoScaler for VM: %s, reason: %v"

	// ErrAutoScalerInfoNotFound error msg
	ErrAutoScalerInfoNotFound = "can't find the VM info from AutoScaler for VM: %s"

	// ErrKubeAdmJoinFailed error msg
	ErrKubeAdmJoinFailed = "unable to join the master kubernetes node for VM: %s, reason: %v"

	// ErrKubeAdmJoinNotRunning error msg
	ErrKubeAdmJoinNotRunning = "could not join kubernetes master node, the VM: %s is not running"

	// ErrStopVMFailed error msg
	ErrStopVMFailed = "could not stop VM: %s, reason: %v"

	// ErrStartVMFailed error msg
	ErrStartVMFailed = "could not start VM: %s, reason: %v"

	// ErrRegisterDNSVMFailed error msg
	ErrRegisterDNSVMFailed = "could not register route53 record VM: %s, reason: %v"

	// ErrDeleteVMFailed error msg
	ErrDeleteVMFailed = "could not delete VM: %s, reason: %v"

	// ErrVMNotFound error msg
	ErrVMNotFound = "unable to find VM: %s"

	// ErrVMStopFailed error msg
	ErrVMStopFailed = "unable to stop VM: %s before delete"

	// ErrProviderIDNotConfigured error msg
	ErrProviderIDNotConfigured = "can't set provider ID for node: %s, reason: %v"

	// ErrPodListReturnError error msg
	ErrPodListReturnError = "unable to list pods on node %s, reason: %v"

	// ErrNodeGroupCleanupFailOnVM error msg
	ErrNodeGroupCleanupFailOnVM = "on node group: %s, failed to delete VM: %s, reason: %v"

	// ErrUncordonNodeReturnError error msg
	ErrUncordonNodeReturnError = "uncordon node: %s got error: %s"

	// ErrCordonNodeReturnError error msg
	ErrCordonNodeReturnError = "cordon node: %s got error: %s"

	// ErrDrainNodeReturnError error msg
	ErrDrainNodeReturnError = "drain node: %s got error: %s"

	// ErrDeleteNodeReturnError error msg
	ErrDeleteNodeReturnError = "delete node: %s got error: %s"

	// ErrLabelNodeReturnError error msg
	ErrLabelNodeReturnError = "set labels on node: %s got error: %s"

	// ErrAnnoteNodeReturnError error msg
	ErrAnnoteNodeReturnError = "set annotations on node: %s got error: %s"

	// ErrKubeCtlIgnoredError error msg
	ErrKubeCtlIgnoredError = "kubectl got error on VM: %s, reason: %s"

	// ErrKubeCtlReturnError error msg
	ErrKubeCtlReturnError = "kubectl got error on VM: %s, %s, reason: %s"

	// ErrNotImplemented error msg
	ErrNotImplemented = "not implemented"

	// ErrNodeIsNotReady error msg
	ErrNodeIsNotReady = "node %s is not ready"

	// ErrUnableToAutoProvisionNodeGroup error msg
	ErrUnableToAutoProvisionNodeGroup = "warning can't autoprovision node group, reason: %v"

	// ErrUnmarshallingError error msg
	ErrUnmarshallingError = "unable to unmarshall node: %s as json, reason: %v"

	// ErrMarshallingError error msg
	ErrMarshallingError = "unable to marshall node: %s as json, reason: %v"

	// ErrKubeletNotConfigured error msg
	ErrKubeletNotConfigured = "can't set provider ID in kubelet for VM: %s, %s, reason: %v"

	// ErrVMNotProvisionnedByMe error msg
	ErrVMNotProvisionnedByMe = "the VM: %s is not provisionned by me"

	// ErrFailedToLoadServerState error msg
	ErrFailedToLoadServerState = "failed to load server state, reason: %v"

	// ErrFailedToSaveServerState error msg
	ErrFailedToSaveServerState = "failed to save server state, reason: %v"

	// ErrRsyncError error msg
	ErrRsyncError = "can't rsync folder for VM: %s, %s, reason: %v"

	// WarnFailedVMNotDeleted warn msg
	WarnFailedVMNotDeleted = "the failed VM:%s is not deleted because status is:%s"

	// ErrWrongStateMachine error msg
	ErrWrongStateMachine = "unexpected instance state %s for instance %s, expected prending or running"

	// ErrWaitIPTimeout error msg
	ErrWaitIPTimeout = "unable to SSH %s after %s seconds"

	// ErrCantCreateVMAlreadyExist error msg
	ErrCantCreateVMAlreadyExist = "unable to create VM %s, because another one exists"

	// ErrPodEvictionAborted
	ErrPodEvictionAborted = "pod eviction aborted"

	// ErrUndefinedPod err msg
	ErrUndefinedPod = "cannot get pod %s/%s, reason: %v"

	// ErrCannotEvictPod err msg
	ErrCannotEvictPod = "cannot evict pod %s/%s, reason: %v"

	// ErrUnableToConfirmPodEviction err msg
	ErrUnableToConfirmPodEviction = "cannot confirm pod %s/%s was deleted, reason: %v"

	// ErrUnableToGetPodListOnNode err msg
	ErrUnableToGetPodListOnNode = "cannot get pods for node %s, reason: %v"

	// ErrUnableEvictAllPodsOnNode err msg
	ErrUnableEvictAllPodsOnNode = "cannot evict all pods on node: %s, reason: %v"

	// ErrTimeoutWhenWaitingEvictions err msg
	ErrTimeoutWhenWaitingEvictions = "timed out waiting for evictions to complete on node: %s"
)
