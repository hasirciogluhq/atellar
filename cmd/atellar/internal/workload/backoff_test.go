package workload

import (
	"testing"
	"time"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

func TestRetryDelay(t *testing.T) {
	want := []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second, 160 * time.Second, 5 * time.Minute, 5 * time.Minute}
	for restarts, d := range want {
		if got := retryDelay(int32(restarts)); got != d {
			t.Errorf("retryDelay(%d) = %s, want %s", restarts, got, d)
		}
	}
	if retryDelay(1000) != backoffMax {
		t.Error("large restart counts must cap")
	}
}

// TestDecide covers status x restart policy x time since the last failure.
func TestDecide(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-5 * time.Second) // inside the first 10s backoff
	old := now.Add(-time.Hour)
	policies := []v1.RestartPolicy{v1.RestartPolicyNo, v1.RestartPolicyOnFailure, v1.RestartPolicyAlways}

	type want struct {
		act     action
		waiting bool
	}
	cases := []struct {
		status v1.ContainerStatus
		since  *time.Time
		want   map[v1.RestartPolicy]want
	}{
		{v1.ContainerStatusPending, nil, all(want{actionStart, false})},
		{v1.ContainerStatusScheduled, nil, all(want{actionStart, false})},
		{v1.ContainerStatusPulling, nil, all(want{actionStart, false})},
		{v1.ContainerStatusCreating, nil, all(want{actionStart, false})},
		{v1.ContainerStatusRunning, nil, all(want{actionVerify, false})},
		{v1.ContainerStatusRemoved, nil, all(want{actionCleanup, false})},
		{v1.ContainerStatusFailed, &old, all(want{actionNone, false})},
		{v1.ContainerStatusTerminated, nil, all(want{actionNone, false})},
		{v1.ContainerStatusStopped, &old, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo: {actionNone, false}, v1.RestartPolicyOnFailure: {actionNone, false}, v1.RestartPolicyAlways: {actionRestart, false},
		}},
		{v1.ContainerStatusStopped, &recent, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo: {actionNone, false}, v1.RestartPolicyOnFailure: {actionNone, false}, v1.RestartPolicyAlways: {actionRestart, true},
		}},
		{v1.ContainerStatusCrashed, &old, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo: {actionNone, false}, v1.RestartPolicyOnFailure: {actionRestart, false}, v1.RestartPolicyAlways: {actionRestart, false},
		}},
		{v1.ContainerStatusCrashed, &recent, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo: {actionNone, false}, v1.RestartPolicyOnFailure: {actionRestart, true}, v1.RestartPolicyAlways: {actionRestart, true},
		}},
		{v1.ContainerStatusBackoff, &recent, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo: {actionNone, false}, v1.RestartPolicyOnFailure: {actionRestart, true}, v1.RestartPolicyAlways: {actionRestart, true},
		}},
		{v1.ContainerStatusBackoff, nil, map[v1.RestartPolicy]want{
			v1.RestartPolicyNo: {actionNone, false}, v1.RestartPolicyOnFailure: {actionRestart, false}, v1.RestartPolicyAlways: {actionRestart, false},
		}},
	}
	for _, tc := range cases {
		for _, policy := range policies {
			c := v1.Container{Status: tc.status, RestartPolicy: policy, LastFailedAt: tc.since, StoppedAt: tc.since}
			act, delay := decide(c, now)
			w := tc.want[policy]
			if act != w.act || (delay > 0) != w.waiting {
				t.Errorf("%s/%s: got %s wait=%s, want %s waiting=%v", tc.status, policy, act, delay, w.act, w.waiting)
			}
		}
	}
}

func all[T any](v T) map[v1.RestartPolicy]T {
	return map[v1.RestartPolicy]T{v1.RestartPolicyNo: v, v1.RestartPolicyOnFailure: v, v1.RestartPolicyAlways: v}
}

func TestAfterFailure(t *testing.T) {
	if afterFailure(v1.RestartPolicyNo) != v1.ContainerStatusFailed ||
		afterFailure(v1.RestartPolicyOnFailure) != v1.ContainerStatusBackoff ||
		afterFailure(v1.RestartPolicyAlways) != v1.ContainerStatusBackoff {
		t.Fatal("afterFailure mapping")
	}
}
