package container

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/application/services"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/application/usecases"
	container "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container"
	containerevent "github.com/hasirciogluhq/atellar/internal/modules/containers/domain/container-event"
	"github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
)

type createContainerRequest struct {
	Image          string            `json:"image" binding:"required"`
	Command        []string          `json:"command"`
	Entrypoint     []string          `json:"entrypoint"`
	Env            map[string]string `json:"env"`
	WorkingDir     *string           `json:"working_dir"`
	CpuLimit       *float64          `json:"cpu_limit"`
	CpuShares      *int32            `json:"cpu_shares"`
	MemoryLimitMiB *int32            `json:"memory_limit_mib"`
	RestartPolicy  string            `json:"restart_policy"`
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

func notifyContainerPeerEvent(ctx context.Context, infra *bootstrap.Infrastructure, event string, target container.Entity) {
	if infra.ContainerPeerNotifier == nil {
		return
	}

	if err := infra.ContainerPeerNotifier.NotifyContainerEvent(ctx, event, target); err != nil {
		log.Printf("container peer notification failed event=%s container_id=%s: %v", event, target.ID, err)
	}
}

func containerStatusPeerEvent(status container.Status) string {
	switch status {
	case container.StatusScheduled, container.StatusPending:
		return agentregistry.PeerEventContainerScheduled
	case container.StatusRunning:
		return agentregistry.PeerEventContainerStarted
	case container.StatusStopped:
		return agentregistry.PeerEventContainerStopped
	case container.StatusTerminated:
		return agentregistry.PeerEventContainerTerminated
	default:
		return ""
	}
}

func registerContainerRoutes(c *gin.RouterGroup, infra *bootstrap.Infrastructure) {
	scheduler := services.NewScheduler(infra.Repositories.Nodes, infra.Repositories.Containers)

	c.POST("", func(ctx *gin.Context) {
		var req createContainerRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		deploy := usecases.NewDeployContainerUseCase(
			scheduler,
			infra.Repositories.Containers,
			infra.ContainerPeerNotifier,
		)
		createdContainer, err := deploy.Execute(ctx.Request.Context(), ports.DeployContainerInput{
			Image:          req.Image,
			Command:        req.Command,
			Entrypoint:     req.Entrypoint,
			Env:            req.Env,
			WorkingDir:     req.WorkingDir,
			CpuLimit:       req.CpuLimit,
			CpuShares:      req.CpuShares,
			MemoryLimitMiB: req.MemoryLimitMiB,
			RestartPolicy:  container.RestartPolicy(req.RestartPolicy),
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusCreated, createdContainer)
	})

	c.DELETE("/:containerId", func(ctx *gin.Context) {
		containerID := ctx.Param("containerId")
		deleteUC := usecases.NewDeleteContainerUseCase(infra.Repositories.Containers, infra.ContainerPeerNotifier)
		removed, err := deleteUC.Execute(ctx.Request.Context(), containerID)
		if err != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, removed)
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

		if event := containerStatusPeerEvent(updatedContainer.Status); event != "" {
			notifyContainerPeerEvent(ctx.Request.Context(), infra, event, *updatedContainer)
		}

		ctx.JSON(http.StatusOK, updatedContainer)
	})

	// Deprecated: agents report via gRPC ReportContainerRuntime.
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

		if req.OverlayIP != "" {
			notifyContainerPeerEvent(ctx.Request.Context(), infra, agentregistry.PeerEventContainerUpdated, *updatedContainer)
		} else if event := containerStatusPeerEvent(updatedContainer.Status); event != "" {
			notifyContainerPeerEvent(ctx.Request.Context(), infra, event, *updatedContainer)
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
