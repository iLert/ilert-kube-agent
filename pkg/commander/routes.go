package commander

import (
	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
)

func SetUpMcpRoutes(router *gin.Engine, cfg *config.Config) {

	router.GET("/api/pod-statuses", AuthorizedHandler(cfg, PodStatusesHandler))
	router.GET("/api/pod-logs", AuthorizedHandler(cfg, PodLogsHandler))
}

func AuthorizedHandler(cfg *config.Config, handler func(*gin.Context, *config.Config)) func(*gin.Context) {
	return func(ctx *gin.Context) {
		if err := CheckAuthorization(ctx, cfg); err != nil {
			return
		}
		handler(ctx, cfg)
	}
}
