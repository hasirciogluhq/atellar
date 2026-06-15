package container

import (
	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
)

func RegisterContainerRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	registerContainerRoutes(c.Group("/containers"), infra)
	registerOverlayIPRoutes(c.Group("/overlay-ips"), infra)
}
