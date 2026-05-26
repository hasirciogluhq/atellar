package http_v1

import (
	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/routes/v1/node"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
)

func RegisterV1Routes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	group := c.Group("/v1")

	node.RegisterNodeRoutes(group, infra)
}
