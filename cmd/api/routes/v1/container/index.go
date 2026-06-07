package container

import (
	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
)

func RegisterContainerRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	registerContainerRoutes(c.Group("/containers"), infra)
	registerOverlayIPRoutes(c.Group("/overlay-ips"), infra)
}
