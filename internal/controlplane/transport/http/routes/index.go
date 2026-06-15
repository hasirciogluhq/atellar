package http_routes

import (
	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	http_v1 "github.com/hasirciogluhq/atellar/internal/controlplane/transport/http/routes/v1"
)

func RegisterRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	http_v1.RegisterV1Routes(c, infra)
}
