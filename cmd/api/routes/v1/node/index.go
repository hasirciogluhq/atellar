package node

import (
	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
)

func RegisterNodeRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	group := c.Group("/nodes")

	registerJoinTokenRoutes(group, infra)
	registerRegisterRoutes(group, infra)
	registerNodeAuthRoutes(group, infra)
	registerHeartbeatRoutes(group, infra)
	registerListNodeRoutes(group, infra)
}
