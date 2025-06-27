package commander

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/apps/v1"
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
	var newPodWaitTimeoutSeconds int64 = 4
	newPodWaitTimeoutSecondsQuery := ctx.Query("newPodWaitTimeoutSeconds")
	newPodWaitTimeoutSecondsValue, err := strconv.ParseInt(newPodWaitTimeoutSecondsQuery, 10, 32)
	if newPodWaitTimeoutSecondsQuery != "" && (err != nil || newPodWaitTimeoutSecondsValue < 0 || newPodWaitTimeoutSecondsValue > 10) {
		log.Warn().Msg("Invalid newPodWaitTimeoutSeconds")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Invalid newPodWaitTimeoutSeconds"})
		return
	} else if newPodWaitTimeoutSecondsQuery != "" {
		newPodWaitTimeoutSeconds = newPodWaitTimeoutSecondsValue
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

	newPodName, err, isPodNotFound := setResourcesByPodName(cfg.KubeClient, namespace, podName, resources, time.Duration(newPodWaitTimeoutSeconds)*time.Second)
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

func setResourcesByPodName(clientset *kubernetes.Clientset, namespace, podName string, resources *ResourceLimits, newPodWaitTimeout time.Duration) (*string, error, bool) {
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
		newPodName, err := setDeploymentResources(clientset, namespace, workload.Name, resources, newPodWaitTimeout)
		return newPodName, err, false
	case WorkloadTypeStatefulSet:
		return nil, setStatefulSetResources(clientset, namespace, workload.Name, resources), false
	default:
		log.Error().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("unsupported workload type")
		return nil, fmt.Errorf("unsupported workload type: %s", workload.Type), false
	}
}

func setDeploymentResources(clientset *kubernetes.Clientset, namespace, deploymentName string, resources *ResourceLimits, newPodWaitTimeout time.Duration) (*string, error) {
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

	chNewPodName := make(chan *string, 1)
	chError := make(chan error, 1)

	go getNewPodName(deployment, currentRS, clientset, newPodWaitTimeout, chNewPodName, chError)

	newPodName := <-chNewPodName
	err = <-chError

	if err != nil {
		log.Warn().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to wait for the new pod name")
		return nil, nil
	}
	return newPodName, nil
}

func getNewPodName(deployment *v1.Deployment, currentRS *v1.ReplicaSet, clientset *kubernetes.Clientset, timeout time.Duration, chPodName chan *string, chError chan error) {
	for start := time.Now(); start.Add(timeout).After(time.Now()); {
		deployment, err := clientset.AppsV1().Deployments(deployment.Namespace).Get(context.TODO(), deployment.Name, metav1.GetOptions{})
		if err != nil {
			log.Error().Err(err).
				Str("deployment_name", deployment.Name).
				Str("namespace", deployment.Namespace).
				Msg("failed to get deployment")
			chPodName <- nil
			chError <- fmt.Errorf("failed to get deployment: %v", err)
			return
		}
		_, _, newRS, err := GetAllReplicaSets(deployment, clientset.AppsV1())
		if err != nil {
			chPodName <- nil
			chError <- err
			return
		}
		if newRS.UID != currentRS.UID {
			log.Info().Str("new pod-template-hash", newRS.Labels["pod-template-hash"]).Str("old pod-template-hash", currentRS.Labels["pod-template-hash"]).Msg("Found new replica set: " + newRS.Name)
			podTemplateHash := newRS.Labels["pod-template-hash"]

			for start.Add(timeout).After(time.Now()) {
				podList, err := clientset.CoreV1().Pods(deployment.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "pod-template-hash=" + podTemplateHash})
				if err != nil {
					log.Warn().Err(err).
						Str("deployment_name", deployment.Name).
						Str("namespace", deployment.Namespace).
						Msg("failed to list pods")
					chPodName <- nil
					chError <- fmt.Errorf("failed to list pods")
					return
				}

				if len(podList.Items) == 0 {
					time.Sleep(time.Second)
					continue
				} else {
					newPodName := podList.Items[0].Name
					log.Info().Interface("newPodName", newPodName).Msg("Found new pod name")
					chPodName <- &newPodName
					chError <- nil
					return
				}
			}
			chPodName <- nil
			chError <- errors.New("timeout before a new replica is found")
			return
		}
		time.Sleep(time.Second)
	}
	chPodName <- nil
	chError <- errors.New("timeout before a new replica is found")
}

func setStatefulSetResources(clientset *kubernetes.Clientset, namespace, statefulSetName string, resources *ResourceLimits) error {
	statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
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

	_, err = clientset.AppsV1().StatefulSets(namespace).Patch(context.TODO(), statefulSetName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
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
