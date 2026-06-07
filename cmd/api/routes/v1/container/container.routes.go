package container

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/application/usecases"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	containerevent "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container-event"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type createContainerRequest struct {
	NodeID         string            `json:"node_id" binding:"required"`
	Image          string            `json:"image" binding:"required"`
	Command        []string          `json:"command"`
	Entrypoint     []string          `json:"entrypoint"`
	Env            map[string]string `json:"env"`
	WorkingDir     *string           `json:"working_dir"`
	CpuLimit       *float64          `json:"cpu_limit"`
	CpuShares      *int32            `json:"cpu_shares"`
	MemoryLimitMiB *int32            `json:"memory_limit_mib"`
	RestartPolicy  string            `json:"restart_policy"`
	ContainerdNs   string            `json:"containerd_ns"`
}

type updateContainerStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type updateContainerRuntimeRequest struct {
	ContainerdID *string `json:"containerd_id"`
	SnapshotKey  *string `json:"snapshot_key"`
	TaskPID      *int32  `json:"task_pid"`
	ImageDigest  *string `json:"image_digest"`
	OverlayIP    string  `json:"overlay_ip"`
	Status       string  `json:"status" binding:"required"`
	ExitCode     *int32  `json:"exit_code"`
	ErrorMessage *string `json:"error_message"`
	RestartCount *int32  `json:"restart_count"`
}

type createContainerEventRequest struct {
	NodeID   string         `json:"node_id" binding:"required"`
	Event    string         `json:"event" binding:"required"`
	Message  *string        `json:"message"`
	Metadata map[string]any `json:"metadata"`
}

func registerContainerRoutes(c *gin.RouterGroup, infra *shared.Infrastructure) {
	c.POST("", func(ctx *gin.Context) {
		var req createContainerRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		restartPolicy := container.RestartPolicy(req.RestartPolicy)
		if restartPolicy == "" {
			restartPolicy = container.RestartPolicyNo
		}

		useCase := usecases.NewCreateContainerUseCase(infra.Repositories.Containers)
		createdContainer, err := useCase.Execute(ctx.Request.Context(), ports.CreateContainerInput{
			NodeID:         req.NodeID,
			Image:          req.Image,
			Command:        req.Command,
			Entrypoint:     req.Entrypoint,
			Env:            req.Env,
			WorkingDir:     req.WorkingDir,
			CpuLimit:       req.CpuLimit,
			CpuShares:      req.CpuShares,
			MemoryLimitMiB: req.MemoryLimitMiB,
			RestartPolicy:  restartPolicy,
			ContainerdNs:   req.ContainerdNs,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusCreated, createdContainer)
	})

	c.GET("", func(ctx *gin.Context) {
		nodeID := ctx.Query("node_id")
		useCase := usecases.NewListContainersUseCase(infra.Repositories.Containers)
		containers, err := useCase.Execute(ctx.Request.Context(), nodeID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, containers)
	})

	c.GET("/:containerId", func(ctx *gin.Context) {
		containerID := ctx.Param("containerId")
		useCase := usecases.NewGetContainerUseCase(infra.Repositories.Containers)
		foundContainer, err := useCase.Execute(ctx.Request.Context(), containerID)
		if err != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, foundContainer)
	})

	c.PATCH("/:containerId/status", func(ctx *gin.Context) {
		containerID := ctx.Param("containerId")
		var req updateContainerStatusRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		useCase := usecases.NewUpdateContainerStatusUseCase(infra.Repositories.Containers)
		updatedContainer, err := useCase.Execute(ctx.Request.Context(), containerID, container.Status(req.Status))
		if err != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, updatedContainer)
	})

	c.PATCH("/:containerId/runtime", func(ctx *gin.Context) {
		containerID := ctx.Param("containerId")
		var req updateContainerRuntimeRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		useCase := usecases.NewUpdateContainerRuntimeUseCase(infra.Repositories.Containers)
		updatedContainer, err := useCase.Execute(ctx.Request.Context(), containerID, ports.UpdateContainerRuntimeInput{
			ContainerdID: req.ContainerdID,
			SnapshotKey:  req.SnapshotKey,
			TaskPID:      req.TaskPID,
			ImageDigest:  req.ImageDigest,
			OverlayIP:    net.ParseIP(req.OverlayIP),
			Status:       container.Status(req.Status),
			ExitCode:     req.ExitCode,
			ErrorMessage: req.ErrorMessage,
			RestartCount: req.RestartCount,
		})
		if err != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, updatedContainer)
	})

	c.GET("/:containerId/events", func(ctx *gin.Context) {
		containerID := ctx.Param("containerId")
		useCase := usecases.NewListContainerEventsUseCase(infra.Repositories.Containers)
		events, err := useCase.Execute(ctx.Request.Context(), containerID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, events)
	})

	c.POST("/:containerId/events", func(ctx *gin.Context) {
		containerID := ctx.Param("containerId")
		var req createContainerEventRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		useCase := usecases.NewCreateContainerEventUseCase(infra.Repositories.Containers)
		event, err := useCase.Execute(ctx.Request.Context(), ports.CreateContainerEventInput{
			ContainerID: containerID,
			NodeID:      req.NodeID,
			Event:       containerevent.EventType(req.Event),
			Message:     req.Message,
			Metadata:    req.Metadata,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusCreated, event)
	})
}
