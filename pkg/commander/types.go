package commander

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

type WorkloadInfo struct {
	Type string // "deployment" or "statefulset"
	Name string
}

type ResourceLimits struct {
	CPULimit      *string `json:"cpuLimit,omitempty"`
	MemoryLimit   *string `json:"memoryLimit,omitempty"`
	CPURequest    *string `json:"cpuRequest,omitempty"`
	MemoryRequest *string `json:"memoryRequest,omitempty"`
}

type Scale struct {
	Replicas int64 `json:"replicas"`
}

type DeleteOptions struct {
	GracePeriodSeconds *int64                      `json:"gracePeriodSeconds,omitempty"`
	PropagationPolicy  *metav1.DeletionPropagation `json:"propagationPolicy,omitempty"`
}
