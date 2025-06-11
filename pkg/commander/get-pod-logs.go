package commander

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPodLogsHandler(ctx *gin.Context, cfg *config.Config) {
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
	previous := false
	if ctx.Query("previous") == "true" {
		previous = true
	}

	req := cfg.KubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Previous: previous,
	})
	podLogs, err := req.Stream()
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to stream logs")
		ctx.String(http.StatusInternalServerError, "Failed to stream logs")
		return
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to copy logs")
		ctx.String(http.StatusInternalServerError, "Failed to copy logs")
		return
	}

	ctx.String(http.StatusOK, buf.String())
}
