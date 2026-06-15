package node

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/modules/nodes/application/usecases"
)

type registerNodeRequest struct {
	Name           string `json:"name"`
	PublicIP       string `json:"public_ip"`
	PrivateIP      string `json:"private_ip"`
	AgentVersion   string `json:"agent_version"`
	ContainerdSock string `json:"containerd_sock"`
}

func registerRegisterRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
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

		useCase := usecases.NewNodeRegisterUseCase(
			infra.Repositories.Nodes,
			infra.OverlayProvisioner,
			infra.NodePeerNotifier,
		)
		node, err := useCase.Execute(c.Request.Context(), usecases.RegisterNodeInput{
			Token:          token,
			Name:           req.Name,
			PublicIP:       net.ParseIP(req.PublicIP),
			PrivateIP:      net.ParseIP(req.PrivateIP),
			AgentVersion:   agentVersion,
			ContainerdSock: req.ContainerdSock,
		})

		if err != nil {
			status := http.StatusInternalServerError
			switch err.Error() {
			case "join token is required", "invalid join token", "join token expired", "join token already used":
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, node)
	})
}
