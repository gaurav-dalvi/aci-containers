package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SnatAllocation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnatAllocationSpec   `json:"spec,omitempty"`
	Status SnatAllocationStatus `json:"status,omitempty"`
}

type SnatAllocationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	PodName       string    `json:"pod_name"`
	NodeName      string    `json:"node_name"`
	SnatPortRange PortRange `json:"snat_port_range,omitempty"`
	SnatIp        string    `json:"snat_ip"`
	Namespace     string    `json:"namespace"`
	MacAddress    string    `json:"mac_address"`
	Scope         string    `json:"string"`
}

type SnatAllocationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// snatAlloction list 
type SnatAllocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SnatAllocation `json:"items"`
}

type PortRange struct {
	Start int `json:"start,omitempty"`
	End   int `json:"end,omitempty"`
}

