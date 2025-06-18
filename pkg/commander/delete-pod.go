package commander

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DeletePodHandler(ctx *gin.Context, cfg *config.Config) {
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

	var gracePeriodSeconds *int64
	gracePeriodSecondsQuery := ctx.Query("gracePeriodSeconds")
	gracePeriodSecondsValue, err := strconv.ParseInt(gracePeriodSecondsQuery, 10, 32)
	if gracePeriodSecondsQuery != "" && (err != nil || gracePeriodSecondsValue < 0) {
		log.Warn().Msg("Invalid gracePeriodSeconds")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Invalid gracePeriodSeconds"})
		return
	}
	if gracePeriodSecondsQuery != "" {
		gracePeriodSeconds = &gracePeriodSecondsValue
	}

	var propagationPolicy *metav1.DeletionPropagation
	propagationPolicyQuery := ctx.Query("propagationPolicy")
	if propagationPolicyQuery != "" && propagationPolicyQuery != string(metav1.DeletePropagationOrphan) && propagationPolicyQuery != string(metav1.DeletePropagationBackground) && propagationPolicyQuery != string(metav1.DeletePropagationForeground) {
		log.Warn().Msg("Invalid propagationPolicy")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Invalid propagationPolicy"})
		return
	}
	if propagationPolicyQuery != "" {
		propagationPolicy = (*metav1.DeletionPropagation)(&propagationPolicyQuery)
	}

	err = cfg.KubeClient.CoreV1().Pods(namespace).Delete(podName, &metav1.DeleteOptions{
		GracePeriodSeconds: gracePeriodSeconds,
		PropagationPolicy:  propagationPolicy,
	})
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to delete pod")
		ctx.PureJSON(http.StatusInternalServerError, gin.H{"message": "Failed to delete pod", "error": err.Error()})
		return
	}

	ctx.PureJSON(http.StatusOK, gin.H{})
}
