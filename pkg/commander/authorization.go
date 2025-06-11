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
		ctx.PureJSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
		return errors.New("unauthorized")
	}

	if authorizationHeader != "Bearer "+cfg.Settings.HttpAuthorizationKey {
		ctx.PureJSON(http.StatusForbidden, gin.H{"message": "Forbidden"})
		return errors.New("incorrect authorization")
	}

	return nil
}
