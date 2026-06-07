package node

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
)

type registerNodeRequest struct {
	Name           string `json:"name"`
	PublicIP       string `json:"public_ip"`
	PrivateIP      string `json:"private_ip"`
	AgentVersion   string `json:"agent_version"`
	ContainerdSock string `json:"containerd_sock"`
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

		var agentVersion *string
		if req.AgentVersion != "" {
			agentVersion = &req.AgentVersion
		}

		useCase := usecases.NewNodeRegisterUseCase(infra.Repositories.Nodes)
		node, err := useCase.Execute(c.Request.Context(), usecases.RegisterNodeInput{
			Token:          token,
			Name:           req.Name,
			PublicIP:       net.ParseIP(req.PublicIP),
			PrivateIP:      net.ParseIP(req.PrivateIP),
			AgentVersion:   agentVersion,
			ContainerdSock: req.ContainerdSock,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, node)
	})
}
