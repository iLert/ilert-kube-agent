package commander

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PodLogsHandler(ctx *gin.Context, cfg *config.Config) {
	podQuery := ctx.Query("pod")
	container := ctx.Query("container")
	tailLinesQuery := ctx.Query("tail-lines")
	sinceSecondsQuery := ctx.Query("since-seconds")
	sinceTimeQuery := ctx.Query("since-time")

	var tailLines *int64
	if lines, err := strconv.ParseInt(tailLinesQuery, 10, 64); err == nil {
		tailLines = &lines
	} else if tailLinesQuery != "" {
		log.Warn().Err(err).Msg(fmt.Sprintf(`Malformed tail-lines "%s"`, tailLinesQuery))
		ctx.String(http.StatusBadRequest, "Malformed tail-lines")
		return
	}

	if sinceSecondsQuery != "" && sinceTimeQuery != "" {
		log.Warn().Msg(fmt.Sprintf(`Both since-seconds and since-time are specified: "%s" & "%s"`, sinceSecondsQuery, sinceTimeQuery))
		ctx.String(http.StatusBadRequest, "Both since-seconds and since-time are specified")
		return
	}

	var sinceSeconds *int64
	if seconds, err := strconv.ParseInt(sinceSecondsQuery, 10, 64); err == nil {
		sinceSeconds = &seconds
	} else if sinceSecondsQuery != "" {
		log.Warn().Err(err).Msg(fmt.Sprintf(`Malformed since-seconds "%s"`, sinceSecondsQuery))
		ctx.String(http.StatusBadRequest, "Malformed since-seconds")
		return
	}

	var sinceTime *metav1.Time
	if time, err := time.Parse(time.RFC3339, sinceTimeQuery); err == nil {
		sinceTime = &metav1.Time{Time: time}
	} else if sinceTimeQuery != "" {
		log.Warn().Err(err).Msg(fmt.Sprintf(`Malformed since-time "%s"`, sinceTimeQuery))
		ctx.String(http.StatusBadRequest, "Malformed since-time")
		return
	}

	pods, err := cfg.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pods from apiserver")
		ctx.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	var selectedPod *v1.Pod
	for _, pod := range pods.Items {
		if pod.Name == podQuery {
			selectedPod = &pod
			break
		}
	}

	if selectedPod == nil {
		log.Warn().Msg(fmt.Sprintf("Pod %s does not exist", podQuery))
		ctx.String(http.StatusBadRequest, "Pod does not exist")
		return
	}

	req := cfg.KubeClient.CoreV1().Pods(selectedPod.Namespace).GetLogs(selectedPod.Name, &v1.PodLogOptions{
		TailLines:    tailLines,
		Container:    container,
		SinceSeconds: sinceSeconds,
		SinceTime:    sinceTime,
	})
	podLogs, err := req.Stream()
	if err != nil {
		log.Error().Err(err).Msg(fmt.Sprintf(`Failed to stream logs, pod: "%s", container: "%s"`, podQuery, container))
		ctx.String(http.StatusInternalServerError, "Internal server error")
		return
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		log.Error().Err(err).Msg(fmt.Sprintf(`Failed to copy logs, pod: "%s", container: "%s"`, podQuery, container))
		ctx.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	ctx.String(http.StatusOK, buf.String())
}
