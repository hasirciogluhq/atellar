package node

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
)

func registerEvictNodeRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	c.POST("/:nodeId/evict", func(c *gin.Context) {
		nodeID := c.Param("nodeId")

		useCase := usecases.NewEvictNodeUseCase(
			infra.Repositories.Nodes,
			infra.NodePeerNotifier,
		)

		evictedNode, err := useCase.Execute(c.Request.Context(), nodeID)
		if err != nil {
			status := http.StatusInternalServerError
			if err.Error() == "node not found" {
				status = http.StatusNotFound
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, evictedNode)
	})
}
