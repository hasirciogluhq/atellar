package node

import (
	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
)

func RegisterNodeRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	group := c.Group("/nodes")

	registerJoinTokenRoutes(group, infra)
	registerRegisterRoutes(group, infra)
	registerNodeAuthRoutes(group, infra)
	registerHeartbeatRoutes(group, infra)
	registerListNodeRoutes(group, infra)
	registerEvictNodeRoutes(group, infra)
	registerNodeOverlayRoutes(group, infra)
}
