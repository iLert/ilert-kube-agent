package commander

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
)

func CheckAuthorization(ctx *gin.Context, cfg *config.Config) error {
	authorizationHeader := ctx.Request.Header.Get("Authorization")
	if authorizationHeader == "" || authorizationHeader != "Bearer "+cfg.Settings.HttpAuthorizationKey {
		ctx.String(http.StatusUnauthorized, "Unauthorized")
		return errors.New("unauthorized")
	}

	return nil
}
