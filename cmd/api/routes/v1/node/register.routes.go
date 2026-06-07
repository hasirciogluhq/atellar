package node

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
)

type registerNodeRequest struct {
	Name      string `json:"name"`
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}

func registerRegisterRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	c.POST("/register", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "token query parameter is required"})
			return
		}

		var req registerNodeRequest
		_ = c.ShouldBindJSON(&req)

		useCase := usecases.NewNodeRegisterUseCase(infra.Repositories.Nodes)
		node, err := useCase.Execute(
			c.Request.Context(),
			token,
			req.Name,
			net.ParseIP(req.PublicIP),
			net.ParseIP(req.PrivateIP),
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, node)
	})
}
