package commander

import (
	"net/http"

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

	deleteOptions := &metav1.DeleteOptions{}
	if err := ctx.ShouldBindJSON(deleteOptions); err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to bind JSON")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to parse request body", "error": err.Error()})
		return
	}

	err := cfg.KubeClient.CoreV1().Pods(namespace).Delete(podName, &metav1.DeleteOptions{
		GracePeriodSeconds: deleteOptions.GracePeriodSeconds,
		PropagationPolicy:  deleteOptions.PropagationPolicy,
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
