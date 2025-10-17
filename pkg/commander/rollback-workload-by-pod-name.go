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

func RollbackWorkloadByPodNameHandler(ctx *gin.Context, cfg *config.Config) {
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

	newPodName, err, isPodNotFound := rollbackWorkloadByPodName(cfg.KubeClient, namespace, podName, time.Duration(waitTimeout)*time.Second)
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

func rollbackWorkloadByPodName(clientset *kubernetes.Clientset, namespace, podName string, waitTimeout time.Duration) (*string, error, bool) {
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
		newPodName, err := rollbackDeployment(clientset, namespace, workload.Name, waitTimeout)
		return newPodName, err, false
	case WorkloadTypeStatefulSet:
		newPodName, err := rollbackStatefulSet(clientset, namespace, workload.Name, waitTimeout)
		return newPodName, err, false
	case WorkloadTypeDaemonSet:
		newPodName, err := rollbackDaemonSet(clientset, namespace, workload.Name, waitTimeout)
		return newPodName, err, false
	default:
		log.Error().
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("unsupported workload type")
		return nil, fmt.Errorf("unsupported workload type: %s", workload.Type), false
	}
}

func rollbackDeployment(clientset *kubernetes.Clientset, namespace, deploymentName string, waitTimeout time.Duration) (*string, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to get deployment")
		return nil, fmt.Errorf("failed to get deployment: %v", err)
	}

	// Get current replica sets to find the previous one
	_, _, currentRS, err := GetAllReplicaSets(deployment, clientset.AppsV1())
	if err != nil {
		return nil, fmt.Errorf("failed to get replica sets: %v", err)
	}

	// Get all replica sets to find the previous one
	rsList, err := clientset.AppsV1().ReplicaSets(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list replica sets: %v", err)
	}

	// Find the previous replica set (not the current one)
	var previousRS *appsv1.ReplicaSet
	for i := range rsList.Items {
		rs := &rsList.Items[i]
		if currentRS != nil && rs.UID == currentRS.UID {
			continue // Skip current replica set
		}
		if metav1.IsControlledBy(rs, deployment) {
			if previousRS == nil || rs.CreationTimestamp.Before(&previousRS.CreationTimestamp) {
				previousRS = rs
			}
		}
	}

	if previousRS == nil {
		return nil, fmt.Errorf("no previous replica set found for rollback")
	}

	// Create rollback patch to revert to previous replica set
	rollbackPatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": previousRS.Spec.Template,
		},
	}

	patchBytes, err := json.Marshal(rollbackPatch)
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to marshal rollback patch")
		return nil, fmt.Errorf("failed to marshal rollback patch: %v", err)
	}

	// Apply the rollback patch
	_, err = clientset.AppsV1().Deployments(namespace).Patch(context.TODO(), deploymentName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to rollback deployment: %v", err)
	}

	// Wait for the new pod to be created
	var newPodName *string
	chNewPodName := make(chan *string, 1)
	chError := make(chan error, 1)
	go getNewPodNameForDeployment(deployment, currentRS, clientset, waitTimeout, chNewPodName, chError)
	newPodName = <-chNewPodName
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

func rollbackStatefulSet(clientset *kubernetes.Clientset, namespace, statefulSetName string, waitTimeout time.Duration) (*string, error) {
	statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), statefulSetName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to get statefulset")
		return nil, fmt.Errorf("failed to get statefulset: %v", err)
	}

	// Get current revision
	currentRevision := statefulSet.Status.CurrentRevision
	if currentRevision == "" {
		return nil, fmt.Errorf("no current revision found for statefulset")
	}

	// Get all controller revisions to find the previous one
	revisionList, err := clientset.AppsV1().ControllerRevisions(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(statefulSet.Spec.Selector),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list controller revisions: %v", err)
	}

	// Find the previous controller revision (not the current one)
	var previousRevision *appsv1.ControllerRevision
	for i := range revisionList.Items {
		revision := &revisionList.Items[i]
		if revision.Name == currentRevision {
			continue // Skip current revision
		}
		if metav1.IsControlledBy(revision, statefulSet) {
			if previousRevision == nil || revision.CreationTimestamp.Before(&previousRevision.CreationTimestamp) {
				previousRevision = revision
			}
		}
	}

	if previousRevision == nil {
		return nil, fmt.Errorf("no previous revision found for rollback")
	}

	// Create rollback patch to revert to previous revision
	rollbackPatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": previousRevision.Data,
		},
	}

	patchBytes, err := json.Marshal(rollbackPatch)
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to marshal rollback patch")
		return nil, fmt.Errorf("failed to marshal rollback patch: %v", err)
	}

	// Apply the rollback patch
	_, err = clientset.AppsV1().StatefulSets(namespace).Patch(context.TODO(), statefulSetName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to rollback statefulset: %v", err)
	}

	// Wait for the new pod to be created
	var newPodName *string
	chNewPodName := make(chan *string, 1)
	chError := make(chan error, 1)
	go getNewPodNameForStatefulSet(statefulSet, currentRevision, clientset, waitTimeout, chNewPodName, chError)
	newPodName = <-chNewPodName
	err = <-chError
	if err != nil {
		log.Warn().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to wait for the new pod name")
		return nil, nil
	}

	return newPodName, nil
}

func rollbackDaemonSet(clientset *kubernetes.Clientset, namespace, daemonSetName string, waitTimeout time.Duration) (*string, error) {
	daemonSet, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("daemonset_name", daemonSetName).
			Str("namespace", namespace).
			Msg("failed to get daemonset")
		return nil, fmt.Errorf("failed to get daemonset: %v", err)
	}

	// Get all controller revisions to find the previous one
	revisionList, err := clientset.AppsV1().ControllerRevisions(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(daemonSet.Spec.Selector),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list controller revisions: %v", err)
	}

	// Find the previous controller revision (not the current one)
	// For DaemonSets, we'll look for the most recent revision that's not the current one
	var previousRevision *appsv1.ControllerRevision
	for i := range revisionList.Items {
		revision := &revisionList.Items[i]
		if metav1.IsControlledBy(revision, daemonSet) {
			// Skip the current revision by checking if it matches the current template
			// Decode the RawExtension to PodTemplateSpec for comparison
			var template corev1.PodTemplateSpec
			if err := json.Unmarshal(revision.Data.Raw, &template); err == nil {
				if equalIgnoreHash(&template, &daemonSet.Spec.Template) {
					continue
				}
			}
			if previousRevision == nil || revision.CreationTimestamp.Before(&previousRevision.CreationTimestamp) {
				previousRevision = revision
			}
		}
	}

	if previousRevision == nil {
		return nil, fmt.Errorf("no previous revision found for rollback")
	}

	// Create rollback patch to revert to previous revision
	rollbackPatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": previousRevision.Data,
		},
	}

	patchBytes, err := json.Marshal(rollbackPatch)
	if err != nil {
		log.Error().Err(err).
			Str("daemonset_name", daemonSetName).
			Str("namespace", namespace).
			Msg("failed to marshal rollback patch")
		return nil, fmt.Errorf("failed to marshal rollback patch: %v", err)
	}

	// Apply the rollback patch
	_, err = clientset.AppsV1().DaemonSets(namespace).Patch(context.TODO(), daemonSetName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to rollback daemonset: %v", err)
	}

	// Wait for the new pod to be created
	var newPodName *string
	chNewPodName := make(chan *string, 1)
	chError := make(chan error, 1)
	go getNewPodNameForDaemonSet(daemonSet, clientset, waitTimeout, chNewPodName, chError)
	newPodName = <-chNewPodName
	err = <-chError
	if err != nil {
		log.Warn().Err(err).
			Str("daemonset_name", daemonSetName).
			Str("namespace", namespace).
			Msg("failed to wait for the new pod name")
		return nil, nil
	}

	return newPodName, nil
}
