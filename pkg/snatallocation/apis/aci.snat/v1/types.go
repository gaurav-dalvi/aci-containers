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
	Podname       string    `json:"podname"`
	Poduid        string `json:"poduid"`
	Nodename      string    `json:"nodename"`
	Snatportrange PortRange `json:"snatportrange"`
	Snatip        string    `json:"snatip"`
	Snatipuid     string  `json:"snatipuid"`
	Namespace     string    `json:"namespace"`
	Macaddress    string    `json:"macaddress"`
	Scope         string    `json:"string,omitempty"`
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

