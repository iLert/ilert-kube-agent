package commander

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ScaleDeploymentHandler(ctx *gin.Context, cfg *config.Config) {
	deploymentName := ctx.Param("deploymentName")
	if deploymentName == "" {
		log.Warn().Msg("Deployment name is required")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Deployment name is required"})
		return
	}
	namespace := ctx.Query("namespace")
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	currentReplicasQuery := ctx.Query("currentReplicas")
	currentReplicas, err := strconv.ParseInt(currentReplicasQuery, 10, 32)
	if currentReplicasQuery != "" && (err != nil || currentReplicas < 0) {
		log.Warn().Msg("Invalid currentReplicas")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Invalid currentReplicas"})
		return
	}

	scale := &Scale{}
	if err := ctx.ShouldBindJSON(scale); err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("Failed to bind JSON")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to parse request body", "error": err.Error()})
		return
	}

	currentScale, err := cfg.KubeClient.AppsV1().Deployments(namespace).GetScale(deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to get deployment scale")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to get deployment scale", "error": err.Error()})
		return
	}

	if currentReplicasQuery != "" && currentReplicas != int64(currentScale.Status.Replicas) {
		ctx.PureJSON(http.StatusAccepted, gin.H{"message": "Precondition failed."})
		return
	}

	currentScale.Spec.Replicas = int32(scale.Replicas)

	_, err = cfg.KubeClient.AppsV1().Deployments(namespace).UpdateScale(deploymentName, currentScale)
	if err != nil {
		log.Error().Err(err).
			Str("deployment_name", deploymentName).
			Str("namespace", namespace).
			Msg("failed to update deployment scale")
		ctx.PureJSON(http.StatusBadRequest, gin.H{"message": "Failed to update deployment scale", "error": err.Error()})
		return
	}

	ctx.PureJSON(http.StatusOK, gin.H{})
}
