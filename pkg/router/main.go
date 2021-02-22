package router

import (
	"github.com/gin-contrib/logger"
	limits "github.com/gin-contrib/size"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/iLert/ilert-kube-agent/pkg/collector"
	"github.com/iLert/ilert-kube-agent/pkg/storage"
)

// Setup init new router
func Setup(srg *storage.Storage) *gin.Engine {

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(logger.SetLogger(logger.Config{
		SkipPath: []string{
			"/api/health",
			"/metrics",
		},
	}))
	router.Use(gin.Recovery())
	router.Use(limits.RequestSizeLimiter(128))

	col := collector.NewCollector(srg)
	prometheus.MustRegister(col)

	prom := promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)
	router.GET("/metrics", func(c *gin.Context) {
		prom.ServeHTTP(c.Writer, c.Request)
		return
	})
	router.GET("/api/health", healthHandler)

	return router
}

func healthHandler(ctx *gin.Context) {
	ctx.PureJSON(200, gin.H{
		"ok": true,
	})
}
