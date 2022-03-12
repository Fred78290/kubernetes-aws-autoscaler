package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedNode is a specification for a ManagedNode resource
type ManagedNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ManagedNodeSpec   `json:"spec,omitempty"`
	Status            ManagedNodeStatus `json:"status,omitempty"`
}

// ManagedNodeNetwork is a specification for a network ManagedNode resource
type ManagedNodeNetwork struct {
	SubnetID           string `json:"subnetID,omitempty"`
	SecurityGroupID    string `json:"securityGroup,omitempty"`
	NetworkInterfaceID string `json:"networkInterfaceID,omitempty"`
	PrivateAddress     string `json:"privateAddress,omitempty"`
	PublicIP           bool   `json:"publicIP,omitempty"`
}

// ManagedNodeSpec is the spec for a ManagedNode resource
type ManagedNodeSpec struct {
	Nodegroup       string              `default:"aws-ca-k8s" json:"nodegroup,omitempty"`
	ControlPlane    bool                `json:"controlPlane,omitempty"`
	AllowDeployment bool                `json:"allowDeployment,omitempty"`
	InstanceType    string              `default:"t2.micro" json:"instanceType"`
	DiskSize        int                 `default:"10240" json:"diskSizeInMb"`
	DiskType        int                 `default:"gp2" json:"diskType"`
	ENI             *ManagedNodeNetwork `json:"eni,omitempty"`
	Labels          []string            `json:"labels,omitempty"`
	Annotations     []string            `json:"annotations,omitempty"`
}

// ManagedNodeStatus is the status for a ManagedNode resource
type ManagedNodeStatus struct {
	// The last time this status was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// The node name created
	NodeName string `json:"nodename,omitempty"`
	// The instance created
	InstanceName string `json:"instancename,omitempty"`
	InstanceID   string `json:"instanceid,omitempty"`
	// A human-readable description of the status of this operation.
	// +optional
	Message string `json:"message,omitempty"`
	// A machine-readable description of why this operation is in the
	// "Failure" status. If this value is empty there
	// is no information available. A Reason clarifies an HTTP status
	// code but does not override it.
	// +optional
	Reason metav1.StatusReason `json:"reason,omitempty"`
	// Suggested HTTP return code for this status, 0 if not set.
	// +optional
	Code int32 `json:"code,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedNodeList is a list of ManagedNode resources
type ManagedNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ManagedNode `json:"items"`
}

func (mn *ManagedNode) GetNodegroup() string {
	return mn.Spec.Nodegroup
}
