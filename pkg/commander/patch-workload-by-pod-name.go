package commander

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func PatchResourcesByPodNameHandler(ctx *gin.Context, cfg *config.Config) {
	podName := ctx.Param("podName")
	if podName == "" {
		log.Warn().Msg("Pod name is required")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Pod name is required"})
		return
	}
	namespace := ctx.Query("namespace")
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	resources := &ResourceLimits{}
	if err := ctx.ShouldBindJSON(resources); err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to bind JSON")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to parse request body", "error": err.Error()})
		return
	}

	if resources.CPULimit == nil && resources.CPURequest == nil && resources.MemoryLimit == nil && resources.MemoryRequest == nil {
		log.Warn().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("At least one resource value is required")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "At least one resource value is required"})
		return
	}

	err := setResourcesByPodName(cfg.KubeClient, namespace, podName, resources)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to set workload resources by pod name")
		ctx.PureJSON(http.StatusInternalServerError, gin.H{"message": "Failed to set workload resources by pod name", "error": err.Error()})
		return
	}

	ctx.PureJSON(http.StatusOK, gin.H{})
}

func setResourcesByPodName(clientset *kubernetes.Clientset, namespace, podName string, resources *ResourceLimits) error {
	workload, err := findWorkloadByPodName(clientset, namespace, podName)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("failed to find workload for pod")
		return fmt.Errorf("failed to find workload for pod %s: %v", podName, err)
	}

	switch workload.Type {
	case "deployment":
		return setDeploymentResources(clientset, namespace, workload.Name, resources)
	case "statefulset":
		return setStatefulSetResources(clientset, namespace, workload.Name, resources)
	default:
		log.Error().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("unsupported workload type")
		return fmt.Errorf("unsupported workload type: %s", workload.Type)
	}
}

func findWorkloadByPodName(clientset *kubernetes.Clientset, namespace, podName string) (*WorkloadInfo, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("failed to get pod")
		return nil, fmt.Errorf("failed to get pod: %v", err)
	}

	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "ReplicaSet":
			rs, err := clientset.AppsV1().ReplicaSets(namespace).Get(owner.Name, metav1.GetOptions{})
			if err != nil {
				log.Error().Err(err).
					Str("pod_name", podName).
					Str("namespace", namespace).
					Msg("failed to get replica set")
				continue
			}
			for _, rsOwner := range rs.OwnerReferences {
				if rsOwner.Kind == "Deployment" {
					return &WorkloadInfo{Type: "deployment", Name: rsOwner.Name}, nil
				}
			}
		case "StatefulSet":
			return &WorkloadInfo{Type: "statefulset", Name: owner.Name}, nil
		case "Deployment":
			return &WorkloadInfo{Type: "deployment", Name: owner.Name}, nil
		}
	}

	return nil, fmt.Errorf("could not determine workload type for pod %s", podName)
}

func setDeploymentResources(clientset *kubernetes.Clientset, namespace, deploymentName string, resources *ResourceLimits) error {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to get deployment")
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	var patches []map[string]interface{}
	for i := range deployment.Spec.Template.Spec.Containers {
		containerPatches := createContainerResourcePatches(i, &deployment.Spec.Template.Spec.Containers[i], resources)
		patches = append(patches, containerPatches...)
	}

	if len(patches) == 0 {
		return fmt.Errorf("no resource changes specified")
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to marshal patches")
		return fmt.Errorf("failed to marshal patches: %v", err)
	}

	_, err = clientset.AppsV1().Deployments(namespace).Patch(deploymentName, types.JSONPatchType, patchBytes)
	return err
}

func setStatefulSetResources(clientset *kubernetes.Clientset, namespace, statefulSetName string, resources *ResourceLimits) error {
	statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(statefulSetName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to get statefulset")
		return fmt.Errorf("failed to get statefulset: %v", err)
	}

	var patches []map[string]interface{}
	for i := range statefulSet.Spec.Template.Spec.Containers {
		containerPatches := createContainerResourcePatches(i, &statefulSet.Spec.Template.Spec.Containers[i], resources)
		patches = append(patches, containerPatches...)
	}

	if len(patches) == 0 {
		return fmt.Errorf("no resource changes specified")
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to marshal patches")
		return fmt.Errorf("failed to marshal patches: %v", err)
	}

	_, err = clientset.AppsV1().StatefulSets(namespace).Patch(statefulSetName, types.JSONPatchType, patchBytes)
	return err
}

func createContainerResourcePatches(containerIndex int, container *corev1.Container, resources *ResourceLimits) []map[string]interface{} {
	var patches []map[string]interface{}

	needsResourcesInit := container.Resources.Limits == nil && container.Resources.Requests == nil
	needsLimitsInit := container.Resources.Limits == nil && (resources.CPULimit != nil || resources.MemoryLimit != nil)
	needsRequestsInit := container.Resources.Requests == nil && (resources.CPURequest != nil || resources.MemoryRequest != nil)

	if needsResourcesInit {
		resourcesValue := make(map[string]interface{})

		if resources.CPULimit != nil || resources.MemoryLimit != nil {
			limits := make(map[string]string)
			if resources.CPULimit != nil {
				limits["cpu"] = *resources.CPULimit
			}
			if resources.MemoryLimit != nil {
				limits["memory"] = *resources.MemoryLimit
			}
			resourcesValue["limits"] = limits
		}

		if resources.CPURequest != nil || resources.MemoryRequest != nil {
			requests := make(map[string]string)
			if resources.CPURequest != nil {
				requests["cpu"] = *resources.CPURequest
			}
			if resources.MemoryRequest != nil {
				requests["memory"] = *resources.MemoryRequest
			}
			resourcesValue["requests"] = requests
		}

		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  fmt.Sprintf("/spec/template/spec/containers/%d/resources", containerIndex),
			"value": resourcesValue,
		})
		return patches
	}

	if needsLimitsInit {
		limits := make(map[string]string)
		if resources.CPULimit != nil {
			limits["cpu"] = *resources.CPULimit
		}
		if resources.MemoryLimit != nil {
			limits["memory"] = *resources.MemoryLimit
		}
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  fmt.Sprintf("/spec/template/spec/containers/%d/resources/limits", containerIndex),
			"value": limits,
		})
	} else {
		if resources.CPULimit != nil {
			patches = append(patches, map[string]interface{}{
				"op":    "replace",
				"path":  fmt.Sprintf("/spec/template/spec/containers/%d/resources/limits/cpu", containerIndex),
				"value": *resources.CPULimit,
			})
		}
		if resources.MemoryLimit != nil {
			patches = append(patches, map[string]interface{}{
				"op":    "replace",
				"path":  fmt.Sprintf("/spec/template/spec/containers/%d/resources/limits/memory", containerIndex),
				"value": *resources.MemoryLimit,
			})
		}
	}

	if needsRequestsInit {
		requests := make(map[string]string)
		if resources.CPURequest != nil {
			requests["cpu"] = *resources.CPURequest
		}
		if resources.MemoryRequest != nil {
			requests["memory"] = *resources.MemoryRequest
		}
		patches = append(patches, map[string]interface{}{
			"op":    "add",
			"path":  fmt.Sprintf("/spec/template/spec/containers/%d/resources/requests", containerIndex),
			"value": requests,
		})
	} else {
		if resources.CPURequest != nil {
			patches = append(patches, map[string]interface{}{
				"op":    "replace",
				"path":  fmt.Sprintf("/spec/template/spec/containers/%d/resources/requests/cpu", containerIndex),
				"value": *resources.CPURequest,
			})
		}
		if resources.MemoryRequest != nil {
			patches = append(patches, map[string]interface{}{
				"op":    "replace",
				"path":  fmt.Sprintf("/spec/template/spec/containers/%d/resources/requests/memory", containerIndex),
				"value": *resources.MemoryRequest,
			})
		}
	}

	return patches
}

func SetOnlyCPULimit(cpuLimit string) ResourceLimits {
	return ResourceLimits{CPULimit: &cpuLimit}
}

func SetOnlyMemoryLimit(memoryLimit string) ResourceLimits {
	return ResourceLimits{MemoryLimit: &memoryLimit}
}

func SetOnlyCPURequest(cpuRequest string) ResourceLimits {
	return ResourceLimits{CPURequest: &cpuRequest}
}

func SetOnlyMemoryRequest(memoryRequest string) ResourceLimits {
	return ResourceLimits{MemoryRequest: &memoryRequest}
}

func SetLimits(cpuLimit, memoryLimit string) ResourceLimits {
	return ResourceLimits{
		CPULimit:    &cpuLimit,
		MemoryLimit: &memoryLimit,
	}
}

func SetRequests(cpuRequest, memoryRequest string) ResourceLimits {
	return ResourceLimits{
		CPURequest:    &cpuRequest,
		MemoryRequest: &memoryRequest,
	}
}

func SetAll(cpuLimit, memoryLimit, cpuRequest, memoryRequest string) ResourceLimits {
	return ResourceLimits{
		CPULimit:      &cpuLimit,
		MemoryLimit:   &memoryLimit,
		CPURequest:    &cpuRequest,
		MemoryRequest: &memoryRequest,
	}
}
