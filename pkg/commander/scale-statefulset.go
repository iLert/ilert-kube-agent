package commander

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ScaleStatefulSetHandler(ctx *gin.Context, cfg *config.Config) {
	statefulSetName := ctx.Param("statefulsetName")
	if statefulSetName == "" {
		log.Warn().Msg("StatefulSet name is required")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "StatefulSet name is required"})
		return
	}
	namespace := ctx.Query("namespace")
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	scale := &Scale{}
	if err := ctx.ShouldBindJSON(scale); err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("Failed to bind JSON")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to parse request body", "error": err.Error()})
		return
	}

	currentScale, err := cfg.KubeClient.AppsV1().StatefulSets(namespace).GetScale(statefulSetName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to get statefulSet scale")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to get statefulSet scale", "error": err.Error()})
		return
	}

	currentScale.Spec.Replicas = int32(scale.Replicas)

	_, err = cfg.KubeClient.AppsV1().StatefulSets(namespace).UpdateScale(statefulSetName, currentScale)
	if err != nil {
		log.Error().Err(err).
			Str("statefulset_name", statefulSetName).
			Str("namespace", namespace).
			Msg("failed to update statefulSet scale")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to update statefulset scale", "error": err.Error()})
		return
	}

	ctx.PureJSON(http.StatusOK, gin.H{})
}
