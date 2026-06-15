package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/hasirciogluhq/atellar/internal/platform/authn"
)

type Action string
type Scope string

const (
	ActionAgentConnect              Action = "agent.connect"
	ActionAgentHeartbeat            Action = "agent.heartbeat"
	ActionAgentReadClusterNetwork   Action = "agent.cluster_network.read"
	ActionAgentReadWorkloads        Action = "agent.workloads.read"
	ActionAgentReportRuntime        Action = "agent.runtime.report"
	ActionAgentAllocateOverlayIP    Action = "agent.overlay_ip.allocate"
	ActionAgentReportHardware       Action = "agent.hardware.report"
	ActionNodeRenewAPIKey           Action = "node.api_key.renew"
	ActionClusterReadNodes          Action = "cluster.nodes.read"
	ActionClusterReadContainers     Action = "cluster.containers.read"
	ActionClusterManageJoinTokens   Action = "cluster.join_tokens.manage"
	ActionClusterManageNodeOverlay  Action = "cluster.node_overlay.manage"
	ActionClusterEvictNode          Action = "cluster.node.evict"
	ActionClusterManageOverlayIPs   Action = "cluster.overlay_ips.manage"
	ActionClusterDeployContainer    Action = "cluster.container.deploy"
	ActionClusterUpdateContainer    Action = "cluster.container.update"
	ActionClusterDeleteContainer    Action = "cluster.container.delete"
	ActionClusterReadContainerEvent Action = "cluster.container_events.read"
)

const (
	ScopeAgentConnect            Scope = "agent:connect"
	ScopeAgentHeartbeat          Scope = "agent:heartbeat"
	ScopeAgentReadNetwork        Scope = "agent:network:read"
	ScopeAgentReadWorkloads      Scope = "agent:workloads:read"
	ScopeAgentReportRuntime      Scope = "agent:runtime:report"
	ScopeAgentAllocateOverlayIP  Scope = "agent:overlay-ip:allocate"
	ScopeAgentReportHardware     Scope = "agent:hardware:report"
	ScopeNodeRenewAPIKey         Scope = "node:api-key:renew"
	ScopeClusterRead             Scope = "cluster:read"
	ScopeClusterAdmin            Scope = "cluster:admin"
	ScopeClusterManageNetworking Scope = "cluster:networking:manage"
	ScopeClusterManageWorkloads  Scope = "cluster:workloads:manage"
)

var (
	ErrMissingPrincipal = errors.New("missing principal")
	ErrForbidden        = errors.New("forbidden")
)

type Policy map[Action][]Scope

type Authorizer struct {
	policy Policy
}

func New(policy Policy) *Authorizer {
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Authorizer{policy: policy}
}

func DefaultPolicy() Policy {
	return Policy{
		ActionAgentConnect:              {ScopeAgentConnect},
		ActionAgentHeartbeat:            {ScopeAgentHeartbeat},
		ActionAgentReadClusterNetwork:   {ScopeAgentReadNetwork},
		ActionAgentReadWorkloads:        {ScopeAgentReadWorkloads},
		ActionAgentReportRuntime:        {ScopeAgentReportRuntime},
		ActionAgentAllocateOverlayIP:    {ScopeAgentAllocateOverlayIP},
		ActionAgentReportHardware:       {ScopeAgentReportHardware},
		ActionNodeRenewAPIKey:           {ScopeNodeRenewAPIKey},
		ActionClusterReadNodes:          {ScopeClusterRead},
		ActionClusterReadContainers:     {ScopeClusterRead},
		ActionClusterManageJoinTokens:   {ScopeClusterAdmin},
		ActionClusterManageNodeOverlay:  {ScopeClusterAdmin, ScopeClusterManageNetworking},
		ActionClusterEvictNode:          {ScopeClusterAdmin},
		ActionClusterManageOverlayIPs:   {ScopeClusterAdmin, ScopeClusterManageNetworking},
		ActionClusterDeployContainer:    {ScopeClusterAdmin, ScopeClusterManageWorkloads},
		ActionClusterUpdateContainer:    {ScopeClusterAdmin, ScopeClusterManageWorkloads},
		ActionClusterDeleteContainer:    {ScopeClusterAdmin, ScopeClusterManageWorkloads},
		ActionClusterReadContainerEvent: {ScopeClusterRead},
	}
}

func NodeAgentScopes() []string {
	return []string{
		string(ScopeAgentConnect),
		string(ScopeAgentHeartbeat),
		string(ScopeAgentReadNetwork),
		string(ScopeAgentReadWorkloads),
		string(ScopeAgentReportRuntime),
		string(ScopeAgentAllocateOverlayIP),
		string(ScopeAgentReportHardware),
		string(ScopeNodeRenewAPIKey),
	}
}

func (a *Authorizer) Assert(ctx context.Context, action Action) error {
	principal, ok := authn.PrincipalFromContext(ctx)
	if !ok || principal == nil {
		return ErrMissingPrincipal
	}
	return a.AssertPrincipal(principal, action)
}

func (a *Authorizer) AssertPrincipal(principal *authn.Principal, action Action) error {
	required, ok := a.policy[action]
	if !ok {
		return fmt.Errorf("%w: action %s has no policy", ErrForbidden, action)
	}
	if len(required) == 0 {
		return nil
	}

	granted := make(map[string]struct{}, len(principal.Scopes))
	for _, scope := range principal.Scopes {
		granted[scope] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := granted[string(scope)]; ok {
			return nil
		}
	}

	return fmt.Errorf("%w: action %s requires one of %v", ErrForbidden, action, required)
}
