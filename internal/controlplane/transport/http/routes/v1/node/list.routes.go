package node

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
)

func registerListNodeRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	c.GET("", func(c *gin.Context) {
		useCase := usecases.NewListNodesUseCase(infra.Repositories.Nodes)
		nodes, err := useCase.Execute(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, nodes)
	})

	c.GET("/:nodeId", func(c *gin.Context) {
		nodeID := c.Param("nodeId")
		useCase := usecases.NewGetNodeUseCase(infra.Repositories.Nodes)
		node, err := useCase.Execute(c.Request.Context(), nodeID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, node)
	})
}
