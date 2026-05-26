package http_routes

import (
	"github.com/gin-gonic/gin"
	http_v1 "github.com/hasirciogluhq/atellar/cmd/api/routes/v1"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
)

func RegisterRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	http_v1.RegisterV1Routes(c, infra)
}
