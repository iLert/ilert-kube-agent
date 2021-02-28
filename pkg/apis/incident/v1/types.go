package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Incident top-level type definition
type Incident struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec IncidentSpec `json:"spec,omitempty"`
}

// IncidentSpec custom spec definition
type IncidentSpec struct {
	ID      int64  `json:"id,omitempty"`
	Summary string `json:"summary,omitempty"`
	Details string `json:"details,omitempty"`
	Type    string `json:"type,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IncidentList list definition
type IncidentList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `son:"metadata,omitempty"`

	Items []Incident `json:"items"`
}
