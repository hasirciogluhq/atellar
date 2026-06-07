package runtime

import (
	"time"
)

const (
	maxRetryAttempts = 5
	backoffBase      = 10 * time.Second
	backoffMax       = 5 * time.Minute
)

func retryDelay(attempt int32) time.Duration {
	if attempt <= 0 {
		return backoffBase
	}
	delay := backoffBase
	for i := int32(0); i < attempt-1; i++ {
		delay *= 2
		if delay > backoffMax {
			return backoffMax
		}
	}
	return delay
}

func shouldRetry(lastFailedAtUnix int64, attempt int32) bool {
	if attempt >= maxRetryAttempts {
		return false
	}
	if lastFailedAtUnix == 0 {
		return true
	}
	last := time.Unix(lastFailedAtUnix, 0)
	return time.Since(last) >= retryDelay(attempt)
}
