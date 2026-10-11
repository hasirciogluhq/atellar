package workload

import (
	"time"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

const (
	backoffBase = 10 * time.Second
	backoffMax  = 5 * time.Minute
)

// retryDelay doubles from backoffBase with every restart, up to backoffMax.
func retryDelay(restarts int32) time.Duration {
	delay := backoffBase
	for i := int32(0); i < restarts; i++ {
		delay *= 2
		if delay >= backoffMax {
			return backoffMax
		}
	}
	return delay
}

type action int

const (
	actionNone action = iota
	actionStart
	actionRestart
	actionVerify
	actionCleanup
)

func (a action) String() string {
	return [...]string{"none", "start", "restart", "verify", "cleanup"}[a]
}

// decide returns what the agent does with a container in its current state
// and, for a restart, how long to wait before it is due.
func decide(c v1.Container, now time.Time) (action, time.Duration) {
	switch c.Status {
	case v1.ContainerStatusRemoved:
		return actionCleanup, 0
	case v1.ContainerStatusPending, v1.ContainerStatusScheduled,
		v1.ContainerStatusPulling, v1.ContainerStatusCreating:
		// Pulling and creating mean an earlier attempt was interrupted.
		return actionStart, 0
	case v1.ContainerStatusRunning:
		return actionVerify, 0
	case v1.ContainerStatusStopped:
		if c.RestartPolicy != v1.RestartPolicyAlways {
			return actionNone, 0
		}
		return actionRestart, wait(c.StoppedAt, c.RestartCount, now)
	case v1.ContainerStatusCrashed, v1.ContainerStatusBackoff:
		if c.RestartPolicy == v1.RestartPolicyNo {
			return actionNone, 0
		}
		return actionRestart, wait(c.LastFailedAt, c.RestartCount, now)
	}
	return actionNone, 0
}

func wait(since *time.Time, restarts int32, now time.Time) time.Duration {
	if since == nil {
		return 0
	}
	return max(since.Add(retryDelay(restarts)).Sub(now), 0)
}

// afterFailure is the status a container takes when starting it failed.
func afterFailure(policy v1.RestartPolicy) v1.ContainerStatus {
	if policy == v1.RestartPolicyNo {
		return v1.ContainerStatusFailed
	}
	return v1.ContainerStatusBackoff
}
