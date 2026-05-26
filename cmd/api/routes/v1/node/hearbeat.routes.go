package node

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	useCases "github.com/hasirciogluhq/atellar/internal/nodes/application/usecases"
)

func registerHeartbeatRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	c.POST("/:nodeId/heartbeat", func(c *gin.Context) {
		nodeID := c.Param("nodeId")
		useCase := useCases.NewNodeHeartbeatUseCase(infra.Repositories.Nodes)
		err := useCase.Execute(c.Request.Context(), nodeID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Node heartbeat updated successfully"})
	})

	c.GET("/:nodeId/heartbeat", func(c *gin.Context) {
		nodeID := c.Param("nodeId")
		useCase := useCases.NewNodeHeartbeatUseCase(infra.Repositories.Nodes)
		err := useCase.Execute(c.Request.Context(), nodeID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Node heartbeat updated successfully"})
	})
}
