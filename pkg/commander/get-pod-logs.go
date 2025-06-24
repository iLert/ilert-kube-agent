package commander

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"

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
	limit := int64(100)
	limitStr := ctx.Query("limit")
	newLimit, err := strconv.ParseInt(limitStr, 10, 64)
	if err == nil && newLimit >= 10 && newLimit <= 500 {
		limit = newLimit
	}

	req := cfg.KubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Previous:  previous,
		TailLines: Int64(limit),
	})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		log.Error().Err(err).
			Str("pod_name", podName).
			Str("namespace", namespace).
			Msg("Failed to stream logs")
		if ErrorMatchers.PodNotFound.Match([]byte(err.Error())) {
			ctx.PureJSON(http.StatusNotFound, gin.H{"message": ErrorPodNotFound})
			return
		}
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

func Int64(v int64) *int64 {
	return &v
}
