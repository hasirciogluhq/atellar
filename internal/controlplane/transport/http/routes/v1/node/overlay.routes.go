package node

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
)

type updateNodeOverlayRequest struct {
	OverlayIP  string `json:"overlay_ip" binding:"required"`
	SubnetCIDR string `json:"overlay_subnet" binding:"required"`
}

func registerNodeOverlayRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	c.PATCH("/:nodeId/overlay", func(c *gin.Context) {
		nodeID := c.Param("nodeId")

		var req updateNodeOverlayRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		useCase := usecases.NewUpdateNodeOverlayUseCase(
			infra.Repositories.Nodes,
			infra.NodePeerNotifier,
		)

		updatedNode, err := useCase.Execute(c.Request.Context(), usecases.UpdateNodeOverlayInput{
			NodeID:     nodeID,
			OverlayIP:  net.ParseIP(req.OverlayIP),
			SubnetCIDR: req.SubnetCIDR,
		})
		if err != nil {
			status := http.StatusInternalServerError
			switch err.Error() {
			case "node not found":
				status = http.StatusNotFound
			case "node id is required", "overlay ip and subnet are required":
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, updatedNode)
	})
}
