package commander

import (
	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
)

func SetUpMcpRoutes(router *gin.Engine, cfg *config.Config) {

	router.GET("/api/pods", AuthorizedHandler(cfg, GetPodsHandler))
	router.GET("/api/pods/:podName", AuthorizedHandler(cfg, GetPodHandler))
	router.GET("/api/pods/:podName/logs", AuthorizedHandler(cfg, GetPodLogsHandler))
	router.PATCH("/api/workloads/:podName", AuthorizedHandler(cfg, PatchResourcesByPodNameHandler))
	router.PATCH("/api/scale/:deploymentName", AuthorizedHandler(cfg, ScaleDeploymentHandler))
}

func AuthorizedHandler(cfg *config.Config, handler func(*gin.Context, *config.Config)) func(*gin.Context) {
	return func(ctx *gin.Context) {
		if err := CheckAuthorization(ctx, cfg); err != nil {
			log.Warn().Err(err).Msg("Authorization failed")
			return
		}
		handler(ctx, cfg)
	}
}
