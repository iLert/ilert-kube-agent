package commander

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PodStatusesHandler(ctx *gin.Context, cfg *config.Config) {
	pods, err := cfg.KubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pods from apiserver")
		ctx.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	podStatuses := make([]PodStatus, 0, len(pods.Items))
	for _, pod := range pods.Items {
		containers := make([]ContainerStatus, 0, len(pod.Status.ContainerStatuses))

		for _, containerStatus := range pod.Status.ContainerStatuses {
			containers = append(containers, ContainerStatus{
				Name:  containerStatus.Name,
				State: containerStatus.State,
				Ready: containerStatus.Ready,
			})
		}

		podStatuses = append(podStatuses, PodStatus{
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			Status:     pod.Status.Phase,
			Containers: containers,
		})
	}

	ctx.PureJSON(http.StatusOK, podStatuses)
}
