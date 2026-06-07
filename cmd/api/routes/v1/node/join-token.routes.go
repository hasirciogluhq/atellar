package node

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
)

type createJoinTokenRequest struct {
	ExpiresAt *time.Time `json:"expires_at"`
	SingleUse *bool      `json:"single_use"`
}

func registerJoinTokenRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	c.POST("/join-tokens", func(c *gin.Context) {
		var req createJoinTokenRequest
		if c.Request.ContentLength > 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		singleUse := true
		if req.SingleUse != nil {
			singleUse = *req.SingleUse
		}

		useCase := usecases.NewCreateJoinTokenUseCase(infra.Repositories.Nodes)
		joinToken, err := useCase.Execute(c.Request.Context(), req.ExpiresAt, singleUse)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, joinToken)
	})

	c.GET("/join-tokens", func(c *gin.Context) {
		useCase := usecases.NewListJoinTokensUseCase(infra.Repositories.Nodes)
		joinTokens, err := useCase.Execute(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, joinTokens)
	})
}
