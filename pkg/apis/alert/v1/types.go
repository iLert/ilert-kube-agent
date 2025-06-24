package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Alert top-level type definition
type Alert struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AlertSpec `json:"spec,omitempty"`
}

// AlertSpec custom spec definition
type AlertSpec struct {
	ID      int64  `json:"id,omitempty"`
	Summary string `json:"summary,omitempty"`
	Details string `json:"details,omitempty"`
	Type    string `json:"type,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AlertList list definition
type AlertList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `son:"metadata,omitempty"`

	Items []Alert `json:"items"`
}
