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

func DeploymentPatchCpuLimitHandler(ctx *gin.Context, cfg *config.Config) {
	container := ctx.Query("container")
	cpuRequest := ctx.Query("cpu-request")
	cpuLimit := ctx.Query("cpu-limit")

	selectedDeployment := getDeployment(ctx, cfg)
	if selectedDeployment == nil {
		return
	}

	patchData := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"` + container + `","resources":{"requests":{"cpu":"` + cpuRequest + `"},"limits":{"cpu":"` + cpuLimit + `"}}}]}}}}`)
	patchDeployment(ctx, cfg, selectedDeployment, patchData)
}

func DeploymentPatchMemoryLimitHandler(ctx *gin.Context, cfg *config.Config) {
	container := ctx.Query("container")
	memoryRequest := ctx.Query("memory-request")
	memoryLimit := ctx.Query("memory-limit")

	selectedDeployment := getDeployment(ctx, cfg)
	if selectedDeployment == nil {
		return
	}

	patchData := []byte(`{"spec":{"template":{"spec":{"containers":[{"name":"` + container + `","resources":{"requests":{"memory":"` + memoryRequest + `","limits":{"memory":"` + memoryLimit + `"}}}}]}}}}`)
	patchDeployment(ctx, cfg, selectedDeployment, patchData)
}

func getDeployment(ctx *gin.Context, cfg *config.Config) *appv1.Deployment {
	namespace := ctx.Query("namespace")
	podName := ctx.Query("pod-name")

	pod, err := cfg.KubeClient.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("Failed to get pod with name '%s' from apiserver", podName)
		log.Warn().Err(err).Msg(msg)
		ctx.String(http.StatusBadRequest, msg+"\n"+err.Error())
		return nil
	}

	podTemplateHash, ok := pod.Labels["pod-template-hash"]
	if !ok {
		msg := fmt.Sprintf("Failed to get pod-template-hash of pod '%s'", podName)
		log.Warn().Msg(msg)
		ctx.String(http.StatusBadRequest, msg)
		return nil
	}

	labelSelector := "pod-template-hash=" + podTemplateHash
	replicaSetList, err := cfg.KubeClient.AppsV1().ReplicaSets(namespace).List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		msg := fmt.Sprintf("Failed to get replica set list with label selector '%s'", labelSelector)
		log.Warn().Err(err).Msg(msg)
		ctx.String(http.StatusBadRequest, msg+"\n"+err.Error())
		return nil
	}

	if len(replicaSetList.Items) == 0 {
		msg := fmt.Sprintf("No replica set with pod-template-hash '%s'", podTemplateHash)
		log.Warn().Msg(msg)
		ctx.String(http.StatusBadRequest, msg)
		return nil
	}
	replicaSet := replicaSetList.Items[0]
	if len(replicaSet.OwnerReferences) == 0 {
		msg := fmt.Sprintf("No ownerships of replica set with name '%s'", replicaSet.Name)
		log.Warn().Msg(msg)
		ctx.String(http.StatusBadRequest, msg)
		return nil
	}

	deploymentReference := replicaSet.OwnerReferences[0]

	deployment, err := cfg.KubeClient.AppsV1().Deployments(metav1.NamespaceAll).Get(deploymentReference.Name, metav1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("Failed to get deployment with name '%s'", deploymentReference.Name)
		log.Warn().Err(err).Msg(msg)
		ctx.String(http.StatusBadRequest, msg+"\n"+err.Error())
		return nil
	}

	return deployment
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
