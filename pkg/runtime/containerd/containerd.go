// Package containerd runs Atellar containers through the containerd API.
package containerd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	eventtypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl/v2"
	"github.com/opencontainers/runtime-spec/specs-go"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

const defaultNamespace = "atellar"

// Runtime talks to one containerd socket in the "atellar" namespace.
type Runtime struct {
	socket    string
	namespace string
	logDir    string
}

func New(socket, logDir string) *Runtime {
	if socket == "" {
		socket = "/run/containerd/containerd.sock"
	}
	if logDir == "" {
		logDir = "/var/log/atellar"
	}
	return &Runtime{socket: socket, namespace: defaultNamespace, logDir: logDir}
}

func (r *Runtime) withClient(ctx context.Context) (context.Context, *containerd.Client, error) {
	client, err := containerd.New(r.socket)
	if err != nil {
		return ctx, nil, err
	}
	return namespaces.WithNamespace(ctx, r.namespace), client, nil
}

func (r *Runtime) EnsureImage(ctx context.Context, imageRef, expectedDigest string) (string, error) {
	ctx, client, err := r.withClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	if expectedDigest != "" {
		if image, err := client.GetImage(ctx, imageRef); err == nil {
			digest := image.Target().Digest.String()
			if digest == expectedDigest || strings.Contains(expectedDigest, digest) {
				return digest, nil
			}
		}
	}

	if image, err := client.GetImage(ctx, imageRef); err == nil {
		return image.Target().Digest.String(), nil
	}

	pulled, err := client.Pull(ctx, imageRef, containerd.WithPullUnpack)
	if err != nil {
		return "", err
	}
	return pulled.Target().Digest.String(), nil
}

type LocalContainer struct {
	ContainerdID string
	SnapshotKey  string
	TaskPID      int32
	ImageDigest  string
	OverlayIP    string
}

func (r *Runtime) RunContainer(ctx context.Context, w *v1.Container, overlayIP string) (*LocalContainer, error) {
	if err := r.PurgeState(ctx, w.ID); err != nil {
		return nil, fmt.Errorf("purge stale state: %w", err)
	}

	ctx, client, err := r.withClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	digest, err := r.EnsureImage(ctx, w.Image, deref(w.ImageDigest))
	if err != nil {
		return nil, err
	}

	image, err := client.GetImage(ctx, w.Image)
	if err != nil {
		return nil, err
	}

	netnsPath := filepath.Join("/run/netns", w.ID)
	specOpts := []oci.SpecOpts{
		oci.WithLinuxNamespace(specs.LinuxNamespace{
			Type: specs.NetworkNamespace,
			Path: netnsPath,
		}),
	}
	if wd := deref(w.WorkingDir); wd != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(wd))
	}
	switch {
	case len(w.Command) > 0 && len(w.Entrypoint) == 0:
		// Override image CMD only; keep image ENTRYPOINT.
		specOpts = append(specOpts, oci.WithImageConfigArgs(image, w.Command))
	case len(w.Entrypoint) > 0:
		specOpts = append(specOpts, oci.WithImageConfig(image))
		args := append([]string{}, w.Entrypoint...)
		args = append(args, w.Command...)
		specOpts = append(specOpts, oci.WithProcessArgs(args...))
	default:
		// Image only: run the image's ENTRYPOINT and CMD.
		specOpts = append(specOpts, oci.WithImageConfig(image))
	}
	if len(w.Env) > 0 {
		env := make([]string, 0, len(w.Env))
		for k, v := range w.Env {
			env = append(env, k+"="+v)
		}
		specOpts = append(specOpts, oci.WithEnv(env))
	}
	specOpts = append(specOpts, resourceOpts(w)...)

	ctr, err := client.NewContainer(ctx, w.ID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(w.ID, image),
		containerd.WithNewSpec(specOpts...),
	)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(r.logDir, 0o755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(r.logDir, w.ID+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	defer logFile.Close()

	task, err := ctr.NewTask(ctx, cio.NewCreator(cio.WithStreams(nil, logFile, logFile)))
	if err != nil {
		return nil, err
	}
	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx)
		return nil, err
	}

	return &LocalContainer{
		ContainerdID: w.ID,
		SnapshotKey:  w.ID,
		TaskPID:      int32(task.Pid()),
		ImageDigest:  digest,
		OverlayIP:    overlayIP,
	}, nil
}

func (r *Runtime) StopAndDelete(ctx context.Context, containerID string) error {
	return r.PurgeState(ctx, containerID)
}

// PurgeState removes containerd container, task, and snapshot for idempotent retries.
func (r *Runtime) PurgeState(ctx context.Context, containerID string) error {
	ctx, client, err := r.withClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	ctr, err := client.LoadContainer(ctx, containerID)
	if err == nil {
		if task, taskErr := ctr.Task(ctx, nil); taskErr == nil {
			_ = task.Kill(ctx, syscall.SIGTERM)
			waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if exitCh, waitErr := task.Wait(waitCtx); waitErr == nil {
				<-exitCh
			}
			cancel()
			_ = task.Kill(ctx, syscall.SIGKILL)
			_, _ = task.Delete(ctx)
		}
		if err := ctr.Delete(ctx, containerd.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	} else if !errdefs.IsNotFound(err) {
		return err
	}

	snapshotter := client.SnapshotService(containerd.DefaultSnapshotter)
	if err := snapshotter.Remove(ctx, containerID); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *Runtime) TaskRunning(ctx context.Context, containerID string) (bool, int32, error) {
	ctx, client, err := r.withClient(ctx)
	if err != nil {
		return false, 0, err
	}
	defer client.Close()

	ctr, err := client.LoadContainer(ctx, containerID)
	if err != nil {
		return false, 0, nil
	}
	task, err := ctr.Task(ctx, nil)
	if err != nil {
		return false, 0, nil
	}
	status, err := task.Status(ctx)
	if err != nil {
		return false, 0, err
	}
	return status.Status == containerd.Running, int32(task.Pid()), nil
}

func (r *Runtime) SubscribeExits(ctx context.Context, handler func(containerID string, exitCode int)) error {
	ctx, client, err := r.withClient(ctx)
	if err != nil {
		return err
	}

	eventsCh, errCh := client.Subscribe(ctx, `topic=="/tasks/exit"`)
	go func() {
		defer client.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errCh:
				if err != nil {
					return
				}
			case ev := <-eventsCh:
				v, err := typeurl.UnmarshalAny(ev.Event)
				if err != nil {
					continue
				}
				if info, ok := v.(*eventtypes.TaskExit); ok {
					handler(info.ContainerID, int(info.ExitStatus))
				}
			}
		}
	}()
	return nil
}

// cpuPeriod is the CFS period used to express fractional CPU limits.
const cpuPeriod = 100_000

// resourceOpts turns the container's limits into cgroup settings.
func resourceOpts(w *v1.Container) []oci.SpecOpts {
	var opts []oci.SpecOpts
	if w.CpuLimit != nil && *w.CpuLimit > 0 {
		opts = append(opts, oci.WithCPUCFS(int64(*w.CpuLimit*cpuPeriod), cpuPeriod))
	}
	if w.CpuShares > 0 {
		opts = append(opts, oci.WithCPUShares(uint64(w.CpuShares)))
	}
	if w.MemoryLimitMiB != nil && *w.MemoryLimitMiB > 0 {
		opts = append(opts, oci.WithMemoryLimit(uint64(*w.MemoryLimitMiB)<<20))
	}
	return opts
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
