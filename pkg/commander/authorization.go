package commander

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iLert/ilert-kube-agent/pkg/config"
)

func CheckAuthorization(ctx *gin.Context, cfg *config.Config) error {
	authorizationHeader := ctx.Request.Header.Get("Authorization")
	if authorizationHeader == "" {
		ctx.String(http.StatusUnauthorized, "Unauthorized")
		return errors.New("unauthorized")
	} else if authorizationHeader != "Bearer "+cfg.Settings.HttpAuthorizationKey {
		ctx.String(http.StatusForbidden, "Forbidden")
		return errors.New("incorrect authorization")
	}

	return nil
}
