package commander

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPodsHandler(ctx *gin.Context, cfg *config.Config) {
	namespace := ctx.Query("namespace")
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	pods, err := cfg.KubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Str("namespace", namespace).Msg("Failed to list pods")
		ctx.PureJSON(http.StatusInternalServerError, gin.H{"message": "Failed to list pods"})
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
