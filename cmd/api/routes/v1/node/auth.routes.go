package node

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/middleware"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
)

func registerNodeAuthRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	auth := c.Group("", middleware.NodeAuth(infra.NodeAuth))

	auth.POST("/me/api-key/renew", func(ctx *gin.Context) {
		credential, err := authn.ParseAuthorizationHeader(ctx.GetHeader(authn.HeaderAuthorization))
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		useCase := usecases.NewRenewNodeAPIKeyUseCase(infra.Repositories.Nodes)
		result, err := useCase.Execute(ctx.Request.Context(), credential.Value)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, result)
	})
}
