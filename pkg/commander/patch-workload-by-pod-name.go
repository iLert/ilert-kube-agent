package commander

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
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
	var waitTimeout int64 = 4
	waitTimeoutQuery := ctx.Query("waitTimeout")
	waitTimeoutValue, err := strconv.ParseInt(waitTimeoutQuery, 10, 32)
	if waitTimeoutQuery != "" && (err != nil || waitTimeoutValue < 0 || waitTimeoutValue > 10) {
		log.Warn().Msg("Invalid waitTimeoutSeconds")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Invalid waitTimeoutSeconds"})
		return
	} else if waitTimeoutQuery != "" {
		waitTimeout = waitTimeoutValue
	}

	resources := &Resources{}
	if err := ctx.ShouldBindJSON(resources); err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to bind JSON")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to parse request body", "error": err.Error()})
		return
	}

	if resources.CPULimit == nil && resources.CPURequest == nil && resources.MemoryLimit == nil && resources.MemoryRequest == nil && resources.Replicas == nil {
		log.Warn().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("At least one resource value is required")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "At least one resource value is required"})
		return
	}

	newPodName, err, isPodNotFound := setResourcesByPodName(cfg.KubeClient, namespace, podName, resources, time.Duration(waitTimeout)*time.Second)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to set workload resources by pod name")
		if isPodNotFound {
			ctx.PureJSON(http.StatusNotFound, gin.H{"message": ErrorPodNotFound})
			return
		}
		ctx.PureJSON(http.StatusInternalServerError, gin.H{"message": "Failed to set workload resources by pod name", "error": err.Error()})
		return
	}

	ctx.PureJSON(http.StatusOK, gin.H{
		"newPodName": newPodName,
	})
}

func setResourcesByPodName(clientset *kubernetes.Clientset, namespace, podName string, resources *Resources, waitTimeout time.Duration) (*string, error, bool) {
	workload, err, isPodNotFound := FindWorkloadByPodName(clientset, namespace, podName)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("failed to find workload for pod")
		return nil, fmt.Errorf("failed to find workload for pod %s: %v", podName, err), isPodNotFound
	}

	switch workload.Type {
	case WorkloadTypeDeployment:
		newPodName, err := setDeploymentResources(clientset, namespace, workload.Name, resources, waitTimeout)
		return newPodName, err, false
	case WorkloadTypeStatefulSet:
		newPodName, err := setStatefulSetResources(clientset, namespace, workload.Name, resources, waitTimeout)
		return newPodName, err, false
	default:
		log.Error().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("unsupported workload type")
		return nil, fmt.Errorf("unsupported workload type: %s", workload.Type), false
	}
}

func setDeploymentResources(clientset *kubernetes.Clientset, namespace, deploymentName string, resources *Resources, waitTimeout time.Duration) (*string, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to get deployment")
		return nil, fmt.Errorf("failed to get deployment: %v", err)
	}

	var patches []map[string]interface{}
	for i := range deployment.Spec.Template.Spec.Containers {
		containerPatches := createContainerResourcePatches(i, &deployment.Spec.Template.Spec.Containers[i], resources)
		patches = append(patches, containerPatches...)
	}
	resourcePatchCount := len(patches)

	if resources.Replicas != nil {
		patches = append(patches, createDeploymentReplicasPatch(deployment, *resources.Replicas))
	}

	if len(patches) == 0 {
		return nil, fmt.Errorf("no resource changes specified")
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to marshal patches")
		return nil, fmt.Errorf("failed to marshal patches: %v", err)
	}

	_, _, currentRS, err := GetAllReplicaSets(deployment, clientset.AppsV1())
	if err != nil {
		return nil, err
	}

	_, err = clientset.AppsV1().Deployments(namespace).Patch(context.TODO(), deploymentName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, err
	}

	var newPodName *string
	chNewPodName := make(chan *string, 1)
	chError := make(chan error, 1)
	if resourcePatchCount > 0 {
		go getNewPodNameForDeployment(deployment, currentRS, clientset, waitTimeout, chNewPodName, chError)
		newPodName = <-chNewPodName
		err = <-chError
	} else {
		go getRunningPodNameForDeployment(deployment, currentRS, clientset, waitTimeout, chNewPodName, chError)
		newPodName = <-chNewPodName
		err = <-chError
	}
	if err != nil {
		log.Warn().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to wait for the new pod name")
		return nil, nil
	}

	return newPodName, nil
}

func setStatefulSetResources(clientset *kubernetes.Clientset, namespace, statefulSetName string, resources *Resources, waitTimeout time.Duration) (*string, error) {
	statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to get statefulset")
		return nil, fmt.Errorf("failed to get statefulset: %v", err)
	}

	var patches []map[string]interface{}
	for i := range statefulSet.Spec.Template.Spec.Containers {
		containerPatches := createContainerResourcePatches(i, &statefulSet.Spec.Template.Spec.Containers[i], resources)
		patches = append(patches, containerPatches...)
	}
	resourcePatchCount := len(patches)

	if resources.Replicas != nil {
		patches = append(patches, createStatefulSetReplicasPatch(statefulSet, *resources.Replicas))
	}

	if len(patches) == 0 {
		return nil, fmt.Errorf("no resource changes specified")
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to marshal patches")
		return nil, fmt.Errorf("failed to marshal patches: %v", err)
	}

	_, err = clientset.AppsV1().StatefulSets(namespace).Patch(context.TODO(), statefulSetName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, err
	}

	var newPodName *string
	chNewPodName := make(chan *string, 1)
	chError := make(chan error, 1)
	if resourcePatchCount > 0 {
		go getNewPodNameForStatefulSet(statefulSet, statefulSet.Status.CurrentRevision, clientset, waitTimeout, chNewPodName, chError)
		newPodName = <-chNewPodName
		err = <-chError
	} else {
		go getRunningPodNameForStatefulSet(statefulSet, statefulSet.Status.CurrentRevision, clientset, waitTimeout, chNewPodName, chError)
		newPodName = <-chNewPodName
		err = <-chError
	}
	if err != nil {
		log.Warn().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to wait for the new pod name")
		return nil, nil
	}

	return newPodName, err
}

func createContainerResourcePatches(containerIndex int, container *corev1.Container, resources *Resources) []map[string]interface{} {
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

func createDeploymentReplicasPatch(deployment *appsv1.Deployment, replicas int64) map[string]interface{} {
	needsReplicasInit := deployment.Spec.Replicas == nil

	if needsReplicasInit {
		return map[string]interface{}{
			"op":    "add",
			"path":  "/spec/replicas",
			"value": replicas,
		}
	} else {
		return map[string]interface{}{
			"op":    "replace",
			"path":  "/spec/replicas",
			"value": replicas,
		}
	}
}

func createStatefulSetReplicasPatch(statefulSet *appsv1.StatefulSet, replicas int64) map[string]interface{} {
	needsReplicasInit := statefulSet.Spec.Replicas == nil

	if needsReplicasInit {
		return map[string]interface{}{
			"op":    "add",
			"path":  "/spec/replicas",
			"value": replicas,
		}
	} else {
		return map[string]interface{}{
			"op":    "replace",
			"path":  "/spec/replicas",
			"value": replicas,
		}
	}
}

func SetOnlyCPULimit(cpuLimit string) Resources {
	return Resources{CPULimit: &cpuLimit}
}

func SetOnlyMemoryLimit(memoryLimit string) Resources {
	return Resources{MemoryLimit: &memoryLimit}
}

func SetOnlyCPURequest(cpuRequest string) Resources {
	return Resources{CPURequest: &cpuRequest}
}

func SetOnlyMemoryRequest(memoryRequest string) Resources {
	return Resources{MemoryRequest: &memoryRequest}
}

func SetLimits(cpuLimit, memoryLimit string) Resources {
	return Resources{
		CPULimit:    &cpuLimit,
		MemoryLimit: &memoryLimit,
	}
}

func SetRequests(cpuRequest, memoryRequest string) Resources {
	return Resources{
		CPURequest:    &cpuRequest,
		MemoryRequest: &memoryRequest,
	}
}

func SetAll(cpuLimit, memoryLimit, cpuRequest, memoryRequest string) Resources {
	return Resources{
		CPULimit:      &cpuLimit,
		MemoryLimit:   &memoryLimit,
		CPURequest:    &cpuRequest,
		MemoryRequest: &memoryRequest,
	}
}
