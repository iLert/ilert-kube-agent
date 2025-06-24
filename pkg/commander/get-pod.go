package commander

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPodHandler(ctx *gin.Context, cfg *config.Config) {
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
	pod, err := cfg.KubeClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to get pod")

		if ErrorMatchers.PodNotFound.Match([]byte(err.Error())) {
			ctx.PureJSON(http.StatusNotFound, gin.H{"message": ErrorPodNotFound})
			return
		}

		ctx.PureJSON(http.StatusInternalServerError, gin.H{"message": "Failed to get pod"})
		return
	}

	ctx.PureJSON(http.StatusOK, pod)
}
