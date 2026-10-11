// Package authz decides whether a principal may perform an action.
package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
)

type Action string
type Scope string

const (
	ActionAgentHeartbeat            Action = "agent.heartbeat"
	ActionAgentReadWorkloads        Action = "agent.workloads.read"
	ActionAgentReportRuntime        Action = "agent.runtime.report"
	ActionAgentAllocateOverlayIP    Action = "agent.overlay_ip.allocate"
	ActionAgentReportHardware       Action = "agent.hardware.report"
	ActionNodeRenewAPIKey           Action = "node.api_key.renew"
	ActionClusterReadNodes          Action = "cluster.nodes.read"
	ActionClusterReadContainers     Action = "cluster.containers.read"
	ActionClusterReadOverlayIPs     Action = "cluster.overlay_ips.read"
	ActionClusterManageJoinTokens   Action = "cluster.join_tokens.manage"
	ActionClusterEvictNode          Action = "cluster.node.evict"
	ActionClusterDeployContainer    Action = "cluster.container.deploy"
	ActionClusterDeleteContainer    Action = "cluster.container.delete"
	ActionClusterReadContainerEvent Action = "cluster.container_events.read"
)

const (
	ScopeAgentHeartbeat         Scope = "agent:heartbeat"
	ScopeAgentReadNetwork       Scope = "agent:network:read"
	ScopeAgentReadWorkloads     Scope = "agent:workloads:read"
	ScopeAgentReportRuntime     Scope = "agent:runtime:report"
	ScopeAgentAllocateOverlayIP Scope = "agent:overlay-ip:allocate"
	ScopeAgentReportHardware    Scope = "agent:hardware:report"
	ScopeNodeRenewAPIKey        Scope = "node:api-key:renew"
	ScopeClusterRead            Scope = "cluster:read"
	ScopeClusterAdmin           Scope = "cluster:admin"
	ScopeClusterManageWorkloads Scope = "cluster:workloads:manage"
)

var (
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("forbidden")
)

// Policy maps an action to the scopes that grant it (any one is enough).
type Policy map[Action][]Scope

func DefaultPolicy() Policy {
	return Policy{
		ActionAgentHeartbeat:            {ScopeAgentHeartbeat},
		ActionAgentReadWorkloads:        {ScopeAgentReadWorkloads},
		ActionAgentReportRuntime:        {ScopeAgentReportRuntime},
		ActionAgentAllocateOverlayIP:    {ScopeAgentAllocateOverlayIP},
		ActionAgentReportHardware:       {ScopeAgentReportHardware},
		ActionNodeRenewAPIKey:           {ScopeNodeRenewAPIKey},
		ActionClusterReadNodes:          {ScopeClusterRead, ScopeAgentReadNetwork},
		ActionClusterReadContainers:     {ScopeClusterRead},
		ActionClusterReadOverlayIPs:     {ScopeClusterRead},
		ActionClusterManageJoinTokens:   {ScopeClusterAdmin},
		ActionClusterEvictNode:          {ScopeClusterAdmin},
		ActionClusterDeployContainer:    {ScopeClusterAdmin, ScopeClusterManageWorkloads},
		ActionClusterDeleteContainer:    {ScopeClusterAdmin, ScopeClusterManageWorkloads},
		ActionClusterReadContainerEvent: {ScopeClusterRead},
	}
}

// NodeAgentScopes are granted to every node API key.
func NodeAgentScopes() []string {
	return []string{
		string(ScopeAgentHeartbeat),
		string(ScopeAgentReadNetwork),
		string(ScopeAgentReadWorkloads),
		string(ScopeAgentReportRuntime),
		string(ScopeAgentAllocateOverlayIP),
		string(ScopeAgentReportHardware),
		string(ScopeNodeRenewAPIKey),
	}
}

// AdminScopes are granted to the cluster admin token.
func AdminScopes() []string {
	return []string{string(ScopeClusterAdmin), string(ScopeClusterRead), string(ScopeClusterManageWorkloads)}
}

type Authorizer struct {
	policy Policy
}

func New(policy Policy) *Authorizer {
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Authorizer{policy: policy}
}

// Authorize checks the principal stored in ctx. It returns
// ErrUnauthenticated for anonymous callers and ErrForbidden otherwise.
func (a *Authorizer) Authorize(ctx context.Context, action Action) error {
	principal := authn.PrincipalFrom(ctx)
	if principal == nil {
		return ErrUnauthenticated
	}
	return a.AuthorizePrincipal(principal, action)
}

func (a *Authorizer) AuthorizePrincipal(principal *authn.Principal, action Action) error {
	required, ok := a.policy[action]
	if !ok {
		return fmt.Errorf("%w: action %s has no policy", ErrForbidden, action)
	}
	for _, have := range principal.Scopes {
		for _, want := range required {
			if have == string(want) {
				return nil
			}
		}
	}
	return fmt.Errorf("%w: action %s requires one of %v", ErrForbidden, action, required)
}
