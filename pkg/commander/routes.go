package commander

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
	"github.com/rs/zerolog/log"
)

func SetUpMcpRoutes(router *gin.Engine, cfg *config.Config) {
	err := ErrorMatchers.Init()
	if err != nil {
		log.Error().Err(err).Msg("Failed to init regexp error matchers.")
	}

	router.GET("/api/pods", AuthorizedHandler(cfg, GetPodsHandler))
	router.GET("/api/pods/:podName", AuthorizedHandler(cfg, GetPodHandler))
	router.GET("/api/pods/:podName/logs", AuthorizedHandler(cfg, GetPodLogsHandler))
	router.PATCH("/api/workloads/:podName", AuthorizedHandler(cfg, PatchResourcesByPodNameHandler))
	router.DELETE("/api/pods/:podName", AuthorizedHandler(cfg, DeletePodHandler))
}

func AuthorizedHandler(cfg *config.Config, handler func(*gin.Context, *config.Config)) func(*gin.Context) {
	return func(ctx *gin.Context) {
		if cfg.Settings.HttpAuthorizationKey == "" {
			log.Warn().Msg("HTTP_AUTHORIZATION_KEY is not set")
			ctx.PureJSON(http.StatusForbidden, gin.H{"message": "HTTP_AUTHORIZATION_KEY is not set"})
			return
		}
		if err := CheckAuthorization(ctx, cfg); err != nil {
			log.Warn().Err(err).Msg("Authorization failed")
			return
		}
		handler(ctx, cfg)
	}
}
