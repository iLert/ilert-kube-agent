package commander

import v1 "k8s.io/api/core/v1"

type PodStatus struct {
	Name      string      `json:"name,omitempty"`
	Namespace string      `json:"namespace,omitempty"`
	Status    v1.PodPhase `json:"phase,omitempty"`
}
