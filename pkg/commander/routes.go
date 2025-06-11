package commander

import (
	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
)

func SetUpMcpRoutes(router *gin.Engine, cfg *config.Config) {

	router.GET("/api/pod-statuses", AuthorizedHandler(cfg, PodStatusesHandler))
	router.GET("/api/pod-logs", AuthorizedHandler(cfg, PodLogsHandler))
	router.GET("/api/deployment-update-cpu-request", AuthorizedHandler(cfg, DeploymentPatchCpuRequestHandler))
	router.GET("/api/deployment-update-memory-request", AuthorizedHandler(cfg, DeploymentPatchMemoryRequestHandler))
	router.GET("/api/deployment-update-cpu-limit", AuthorizedHandler(cfg, DeploymentPatchCpuLimitHandler))
	router.GET("/api/deployment-update-memory-limit", AuthorizedHandler(cfg, DeploymentPatchMemoryLimitHandler))
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
