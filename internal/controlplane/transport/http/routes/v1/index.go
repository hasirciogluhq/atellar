package http_v1

import (
	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/controlplane/transport/http/routes/v1/container"
	"github.com/hasirciogluhq/atellar/internal/controlplane/transport/http/routes/v1/node"
)

func RegisterV1Routes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	group := c.Group("/v1")

	node.RegisterNodeRoutes(group, infra)
	container.RegisterContainerRoutes(group, infra)
}
