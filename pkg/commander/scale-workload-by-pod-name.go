package commander

import (
	"context"
	"fmt"
	"net/http"

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

	scale := &Scale{}
	if err := ctx.ShouldBindJSON(scale); err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to bind JSON")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to parse request body", "error": err.Error()})
		return
	}

	err, isPodNotFound := scaleWorkloadByPodName(cfg.KubeClient, namespace, podName, scale.Replicas)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to scale resource by pod name")
		if isPodNotFound {
			ctx.PureJSON(http.StatusNotFound, gin.H{"message": ErrorPodNotFound})
			return
		}
		ctx.PureJSON(http.StatusInternalServerError, gin.H{"message": "Failed to scale resource", "error": err.Error()})
		return
	}

	ctx.PureJSON(http.StatusOK, gin.H{})
}

func scaleWorkloadByPodName(clientset *kubernetes.Clientset, namespace, podName string, replicas int64) (error, bool) {
	workload, err, isPodNotFound := FindWorkloadByPodName(clientset, namespace, podName)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("failed to find workload for pod")
		return fmt.Errorf("failed to find workload for pod %s: %v", podName, err), isPodNotFound
	}

	switch workload.Type {
	case WorkloadTypeDeployment:
		return scaleDeployment(clientset, namespace, workload.Name, replicas), false
	case WorkloadTypeStatefulSet:
		return scaleStatefulSet(clientset, namespace, workload.Name, replicas), false
	default:
		log.Error().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("unsupported workload type")
		return fmt.Errorf("unsupported workload type: %s", workload.Type), false
	}
}

func scaleDeployment(clientset *kubernetes.Clientset, namespace, deploymentName string, replicas int64) error {
	currentScale, err := clientset.AppsV1().Deployments(namespace).GetScale(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to get deployment scale")
		return fmt.Errorf("failed to get deployment scale: %v", err)
	}

	currentScale.Spec.Replicas = int32(replicas)

	_, err = clientset.AppsV1().Deployments(namespace).UpdateScale(context.TODO(), deploymentName, currentScale, metav1.UpdateOptions{})
	return err
}

func scaleStatefulSet(clientset *kubernetes.Clientset, namespace, statefulSetName string, replicas int64) error {
	currentScale, err := clientset.AppsV1().StatefulSets(namespace).GetScale(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to get statefulset scale")
		return fmt.Errorf("failed to get statefulset scale: %v", err)
	}

	currentScale.Spec.Replicas = int32(replicas)

	_, err = clientset.AppsV1().StatefulSets(namespace).UpdateScale(context.TODO(), statefulSetName, currentScale, metav1.UpdateOptions{})
	return err
}
