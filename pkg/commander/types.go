package commander

import v1 "k8s.io/api/core/v1"

type PodStatus struct {
	Name       string            `json:"name,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	Status     v1.PodPhase       `json:"phase,omitempty"`
	Containers []ContainerStatus `json:"containers"`
}

type ContainerStatus struct {
	Name  string            `json:"name"`
	State v1.ContainerState `json:"state,omitempty"`
	Ready bool              `json:"ready"`
}

type ContainerResourceRequirements struct {
	Name      string                  `json:"name"`
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
}
