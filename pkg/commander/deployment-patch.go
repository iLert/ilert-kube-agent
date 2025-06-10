package commander

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func DeploymentPatchCpuRequestHandler(ctx *gin.Context, cfg *config.Config) {
	container := ctx.Query("container")
	cpuRequest := ctx.Query("cpu-request")

	selectedDeployment := getDeployment(ctx, cfg)
	if selectedDeployment == nil {
		return
	}

	patchData := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"` + container + `","resources":{"requests":{"cpu":"` + cpuRequest + `"}}}]}}}}`)
	patchDeployment(ctx, cfg, selectedDeployment, patchData)
}

func DeploymentPatchMemoryRequestHandler(ctx *gin.Context, cfg *config.Config) {
	container := ctx.Query("container")
	memoryRequest := ctx.Query("memory-request")

	selectedDeployment := getDeployment(ctx, cfg)
	if selectedDeployment == nil {
		return
	}

	patchData := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"` + container + `","resources":{"requests":{"memory":"` + memoryRequest + `"}}}]}}}}`)
	patchDeployment(ctx, cfg, selectedDeployment, patchData)
}

func DeploymentPatchCpuLimitHandler(ctx *gin.Context, cfg *config.Config) {
	container := ctx.Query("container")
	cpuLimit := ctx.Query("cpu-limit")

	selectedDeployment := getDeployment(ctx, cfg)
	if selectedDeployment == nil {
		return
	}

	patchData := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"` + container + `","resources":{"limits":{"cpu":"` + cpuLimit + `"}}}]}}}}`)
	patchDeployment(ctx, cfg, selectedDeployment, patchData)
}

func DeploymentPatchMemoryLimitHandler(ctx *gin.Context, cfg *config.Config) {
	container := ctx.Query("container")
	memoryLimit := ctx.Query("memory-limit")

	selectedDeployment := getDeployment(ctx, cfg)
	if selectedDeployment == nil {
		return
	}

	patchData := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"` + container + `","resources":{"limits":{"memory":"` + memoryLimit + `"}}}]}}}}`)
	patchDeployment(ctx, cfg, selectedDeployment, patchData)
}

func getDeployment(ctx *gin.Context, cfg *config.Config) *appv1.Deployment {
	deploymentQuery := ctx.Query("deployment")

	deployments, err := cfg.KubeClient.AppsV1().Deployments(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get deployments from apiserver")
		ctx.String(http.StatusInternalServerError, "Internal server error")
		return nil
	}

	var selectedDeployment *appv1.Deployment
	for _, deployment := range deployments.Items {
		if deployment.Name == deploymentQuery {
			selectedDeployment = &deployment
			break
		}
	}

	if selectedDeployment == nil {
		log.Warn().Msg(fmt.Sprintf("Deployment %s does not exist", deploymentQuery))
		ctx.String(http.StatusBadRequest, "Deployment does not exist")
		return nil
	}

	return selectedDeployment
}

func patchDeployment(ctx *gin.Context, cfg *config.Config, deployment *appv1.Deployment, patchData []byte) {
	deploymentQuery := ctx.Query("deployment")

	if !json.Valid(patchData) {
		log.Error().Msg(fmt.Sprintf(`Invalid patch :"%s"`, string(patchData)))
		ctx.String(http.StatusInternalServerError, "Internal server error")
		return
	}

	result, err := cfg.KubeClient.AppsV1().Deployments(deployment.Namespace).Patch(deployment.Name, types.StrategicMergePatchType, patchData)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "Invalid value:") {
			log.Warn().Err(err).Msg(fmt.Sprintf(`Failed to patch deployment: "%s", patch: "%s"`, deploymentQuery, string(patchData)))
			ctx.String(http.StatusBadRequest, msg)
		} else {
			log.Error().Err(err).Msg(fmt.Sprintf(`Failed to patch deployment: "%s", patch: "%s"`, deploymentQuery, string(patchData)))
			ctx.String(http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	containers := make([]ContainerResourceRequirements, 0, len(result.Spec.Template.Spec.Containers))
	for _, container := range result.Spec.Template.Spec.Containers {
		containers = append(containers, ContainerResourceRequirements{
			Name:      container.Name,
			Resources: container.Resources,
		})
	}

	ctx.PureJSON(http.StatusOK, containers)
}
