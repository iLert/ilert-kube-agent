package commander

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ScaleWorkloadByPodNameHandler(ctx *gin.Context, cfg *config.Config) {
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
	currentReplicasQuery := ctx.Query("currentReplicas")
	currentReplicas, err := strconv.ParseInt(currentReplicasQuery, 10, 32)
	if currentReplicasQuery != "" && (err != nil || currentReplicas < 0) {
		log.Warn().Msg("Invalid currentReplicas")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Invalid currentReplicas"})
		return
	}

	scale := &Scale{}
	if err := ctx.ShouldBindJSON(scale); err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to bind JSON")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to parse request body", "error": err.Error()})
		return
	}

	if currentReplicasQuery == "" {
		currentReplicas = -1
	}

	err = scaleWorkloadByPodName(cfg.KubeClient, namespace, podName, currentReplicas, scale.Replicas)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to scale resource by pod name")
		ctx.PureJSON(http.StatusInternalServerError, gin.H{"message": "Failed to scale resource", "error": err.Error()})
		return
	}

	ctx.PureJSON(http.StatusOK, gin.H{})
}

func scaleWorkloadByPodName(clientset *kubernetes.Clientset, namespace, podName string, currentReplicas int64, replicas int64) error {
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
		return scaleDeployment(clientset, namespace, workload.Name, currentReplicas, replicas)
	case "statefulset":
		return scaleStatefulSet(clientset, namespace, workload.Name, currentReplicas, replicas)
	default:
		log.Error().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("unsupported workload type")
		return fmt.Errorf("unsupported workload type: %s", workload.Type)
	}
}

func scaleDeployment(clientset *kubernetes.Clientset, namespace, deploymentName string, currentReplicas int64, replicas int64) error {
	currentScale, err := clientset.AppsV1().Deployments(namespace).GetScale(deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to get deployment scale")
		return fmt.Errorf("failed to get deployment scale: %v", err)
	}

	if currentReplicas >= 0 && currentReplicas != int64(currentScale.Status.Replicas) {
		return fmt.Errorf("precondition failed")
	}

	currentScale.Spec.Replicas = int32(replicas)

	_, err = clientset.AppsV1().Deployments(namespace).UpdateScale(deploymentName, currentScale)
	return err
}

func scaleStatefulSet(clientset *kubernetes.Clientset, namespace, statefulSetName string, currentReplicas int64, replicas int64) error {
	currentScale, err := clientset.AppsV1().StatefulSets(namespace).GetScale(statefulSetName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to get statefulset scale")
		return fmt.Errorf("failed to get statefulset scale: %v", err)
	}

	if currentReplicas >= 0 && currentReplicas != int64(currentScale.Status.Replicas) {
		return fmt.Errorf("precondition failed")
	}

	currentScale.Spec.Replicas = int32(replicas)

	_, err = clientset.AppsV1().StatefulSets(namespace).UpdateScale(statefulSetName, currentScale)
	return err
}
