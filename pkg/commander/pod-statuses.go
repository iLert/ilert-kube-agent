package commander

import (
	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PodStatusesHandler(ctx *gin.Context, cfg *config.Config) {
	podInterface := cfg.KubeClient.CoreV1().Pods(metav1.NamespaceAll)
	pods, err := podInterface.List(metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pods from apiserver")
	}

	podStatuses := make([]PodStatus, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podStatuses = append(podStatuses, PodStatus{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    pod.Status.Phase,
		})
	}

	ctx.PureJSON(200, podStatuses)
}
