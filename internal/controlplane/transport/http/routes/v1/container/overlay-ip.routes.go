package container

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/application/usecases"
)

type createOverlayIPRequest struct {
	IP     string `json:"ip" binding:"required"`
	NodeID string `json:"node_id" binding:"required"`
}

type allocateOverlayIPRequest struct {
	ContainerID string `json:"container_id" binding:"required"`
}

func registerOverlayIPRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	c.POST("", func(ctx *gin.Context) {
		var req createOverlayIPRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		useCase := usecases.NewCreateOverlayIPPoolEntryUseCase(infra.Repositories.Containers)
		entry, err := useCase.Execute(ctx.Request.Context(), net.ParseIP(req.IP), req.NodeID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusCreated, entry)
	})

	c.GET("", func(ctx *gin.Context) {
		nodeID := ctx.Query("node_id")
		if nodeID == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "node_id query parameter is required"})
			return
		}

		freeOnly := ctx.Query("free") == "true"
		useCase := usecases.NewListOverlayIPsUseCase(infra.Repositories.Containers)
		entries, err := useCase.Execute(ctx.Request.Context(), nodeID, freeOnly)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, entries)
	})

	c.POST("/:ip/allocate", func(ctx *gin.Context) {
		ip := net.ParseIP(ctx.Param("ip"))
		var req allocateOverlayIPRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		useCase := usecases.NewAllocateOverlayIPUseCase(infra.Repositories.Containers)
		entry, err := useCase.Execute(ctx.Request.Context(), ip, req.ContainerID)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, entry)
	})

	c.POST("/:ip/release", func(ctx *gin.Context) {
		ip := net.ParseIP(ctx.Param("ip"))
		useCase := usecases.NewReleaseOverlayIPUseCase(infra.Repositories.Containers)
		if err := useCase.Execute(ctx.Request.Context(), ip); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": "overlay ip released"})
	})
}
