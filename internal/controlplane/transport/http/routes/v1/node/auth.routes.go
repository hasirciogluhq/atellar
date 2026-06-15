package node

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/controlplane/transport/http/middleware"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
	"github.com/hasirciogluhq/atellar/internal/platform/authz"
)

func registerNodeAuthRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	auth := c.Group("", middleware.NodeAuth(infra.NodeAuth))

	auth.POST("/me/api-key/renew", middleware.Require(infra.Authz, authz.ActionNodeRenewAPIKey), func(ctx *gin.Context) {
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
