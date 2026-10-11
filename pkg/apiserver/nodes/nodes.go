// Package nodes implements api.NodeInterface: registration, heartbeats,
// eviction and overlay subnet provisioning.
package nodes

import (
	"context"
	"errors"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/registry"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/auth/authz"
	"github.com/hasirciogluhq/atellar/pkg/auth/token"
	"github.com/hasirciogluhq/atellar/pkg/network/ipam"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

type Registry struct {
	deps      *registry.Deps
	allocator *ipam.Allocator
	now       func() time.Time
}

var _ api.NodeInterface = (*Registry)(nil)

func New(deps *registry.Deps, allocator *ipam.Allocator) *Registry {
	return &Registry{deps: deps, allocator: allocator, now: time.Now}
}

func (r *Registry) List(ctx context.Context) ([]v1.Node, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterReadNodes); err != nil {
		return nil, err
	}
	nodes, err := r.deps.Store.ListNodes(ctx)
	return nodes, registry.StorageError(err, "nodes", "")
}

func (r *Registry) Get(ctx context.Context, ref string) (*v1.Node, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterReadNodes); err != nil {
		return nil, err
	}
	return r.resolve(ctx, ref)
}

func (r *Registry) Watch(ctx context.Context) (watch.Interface[v1.Node], error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterReadNodes); err != nil {
		return nil, err
	}
	return registry.Watch(ctx, r.deps.NodeEvents, nil, func(ctx context.Context) ([]v1.Node, error) {
		nodes, err := r.deps.Store.ListNodes(ctx)
		return nodes, registry.StorageError(err, "nodes", "")
	})
}

// Evict takes a node out of the cluster. Its subnet stays reserved until a
// new node registers and reclaims it.
func (r *Registry) Evict(ctx context.Context, ref string) (*v1.Node, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterEvictNode); err != nil {
		return nil, err
	}
	node, err := r.resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if node.Status == v1.NodeStatusEvicted {
		return node, nil
	}
	evicted, err := r.deps.Store.EvictNode(ctx, node.ID)
	if err != nil {
		return nil, registry.StorageError(err, "node", ref)
	}
	r.deps.NodeEvents.Publish(watch.Modified, *evicted)
	return evicted, nil
}

func (r *Registry) ListOverlayIPs(ctx context.Context, ref string, freeOnly bool) ([]v1.OverlayIP, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterReadOverlayIPs); err != nil {
		return nil, err
	}
	node, err := r.resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	ips, err := r.deps.Store.ListOverlayIPs(ctx, node.ID, freeOnly)
	return ips, registry.StorageError(err, "node", ref)
}

// Register consumes a join token, creates the node, assigns its overlay
// subnet and issues its API key, all in one transaction.
func (r *Registry) Register(ctx context.Context, req v1.RegisterNodeRequest) (*v1.RegisterNodeResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, registry.BadRequest(err)
	}

	var resp *v1.RegisterNodeResponse
	err := r.deps.Store.Tx(ctx, func(tx storage.Interface) error {
		jt, err := tx.GetJoinTokenByHash(ctx, token.Hash(req.JoinToken))
		if errors.Is(err, storage.ErrNotFound) {
			return api.BadRequest("invalid join token")
		}
		if err != nil {
			return registry.StorageError(err, "join token", "")
		}
		if jt.ExpiresAt != nil && jt.ExpiresAt.Before(r.now()) {
			return api.BadRequest("join token expired")
		}
		if jt.SingleUse && jt.UsedAt != nil {
			return api.BadRequest("join token already used")
		}

		node, err := tx.CreateNode(ctx, storage.CreateNodeParams{
			Name:           req.Name,
			PublicIP:       req.PublicIP,
			PrivateIP:      req.PrivateIP,
			AgentVersion:   optional(req.AgentVersion),
			ContainerdSock: req.ContainerdSock,
		})
		if errors.Is(err, storage.ErrConflict) {
			return api.Conflict("node name %q is already registered", req.Name)
		}
		if err != nil {
			return registry.StorageError(err, "node", req.Name)
		}

		if err := tx.MarkJoinTokenUsed(ctx, jt.ID, node.ID); errors.Is(err, storage.ErrConflict) {
			return api.BadRequest("join token already used")
		} else if err != nil {
			return registry.StorageError(err, "join token", jt.ID)
		}

		node, err = r.provisionOverlay(ctx, tx, node.ID)
		if err != nil {
			return err
		}

		key, err := issueAPIKey(ctx, tx, node.ID, r.now())
		if err != nil {
			return err
		}
		resp = &v1.RegisterNodeResponse{Node: *node, NodeAPIKey: key.NodeAPIKey, APIKeyExpiresAt: key.APIKeyExpiresAt}
		return nil
	})
	if err != nil {
		return nil, err
	}

	r.deps.NodeEvents.Publish(watch.Added, resp.Node)
	return resp, nil
}

// provisionOverlay assigns the node a subnet from the cluster CIDR. A subnet
// left by an evicted node is reused first.
func (r *Registry) provisionOverlay(ctx context.Context, tx storage.Interface, nodeID string) (*v1.Node, error) {
	if err := tx.LockOverlayAllocation(ctx); err != nil {
		return nil, registry.StorageError(err, "overlay", nodeID)
	}
	reclaimable, err := tx.ListReclaimableOverlaySubnets(ctx)
	if err != nil {
		return nil, registry.StorageError(err, "overlay", nodeID)
	}
	active, err := tx.ListActiveOverlaySubnets(ctx)
	if err != nil {
		return nil, registry.StorageError(err, "overlay", nodeID)
	}

	candidates := make([]ipam.ReclaimableSubnet, 0, len(reclaimable))
	for _, c := range reclaimable {
		candidates = append(candidates, ipam.ReclaimableSubnet{SourceNodeID: c.NodeID, SubnetCIDR: c.SubnetCIDR})
	}
	result, err := r.allocator.Allocate(candidates, active)
	if errors.Is(err, ipam.ErrNoSubnetsAvailable) {
		return nil, api.Unavailable("%v", err)
	}
	if err != nil {
		return nil, api.Internal("allocate overlay subnet: %v", err)
	}

	if result.SourceNodeID != "" {
		if err := tx.DeleteOverlayPool(ctx, result.SourceNodeID); err != nil {
			return nil, registry.StorageError(err, "overlay pool", result.SourceNodeID)
		}
		if err := tx.ClearNodeOverlay(ctx, result.SourceNodeID); err != nil {
			return nil, registry.StorageError(err, "node", result.SourceNodeID)
		}
	}

	alloc := result.Allocation
	node, err := tx.SetNodeOverlay(ctx, nodeID, alloc.NodeIP.String(), alloc.SubnetCIDR, v1.NodeStatusReady)
	if err != nil {
		return nil, registry.StorageError(err, "node", nodeID)
	}
	pool := make([]string, 0, len(alloc.PoolIPs))
	for _, ip := range alloc.PoolIPs {
		pool = append(pool, ip.String())
	}
	if err := tx.SeedOverlayPool(ctx, nodeID, pool); err != nil {
		return nil, registry.StorageError(err, "overlay pool", nodeID)
	}
	return node, nil
}

// SelfHeartbeat records liveness. A node marked not_ready by the lifecycle
// controller becomes ready again; only that transition is published, so
// heartbeats do not flood watchers.
func (r *Registry) SelfHeartbeat(ctx context.Context) error {
	self, err := r.self(ctx, authz.ActionAgentHeartbeat)
	if err != nil {
		return err
	}
	node, err := r.deps.Store.HeartbeatNode(ctx, self.ID)
	if err != nil {
		return registry.StorageError(err, "node", self.ID)
	}
	if node.Status != v1.NodeStatusNotReady {
		return nil
	}
	node, err = r.deps.Store.SetNodeStatus(ctx, self.ID, v1.NodeStatusReady)
	if err != nil {
		return registry.StorageError(err, "node", self.ID)
	}
	r.deps.NodeEvents.Publish(watch.Modified, *node)
	return nil
}

func (r *Registry) SelfReportHardware(ctx context.Context, hw v1.NodeHardware) error {
	self, err := r.self(ctx, authz.ActionAgentReportHardware)
	if err != nil {
		return err
	}
	if err := hw.Validate(); err != nil {
		return registry.BadRequest(err)
	}
	node, err := r.deps.Store.UpdateNodeHardware(ctx, self.ID, hw)
	if err != nil {
		return registry.StorageError(err, "node", self.ID)
	}
	r.deps.NodeEvents.Publish(watch.Modified, *node)
	return nil
}

// SelfRenewAPIKey replaces the caller's key; the old key stops working.
func (r *Registry) SelfRenewAPIKey(ctx context.Context) (*v1.NodeAPIKey, error) {
	self, err := r.self(ctx, authz.ActionNodeRenewAPIKey)
	if err != nil {
		return nil, err
	}
	return issueAPIKey(ctx, r.deps.Store, self.ID, r.now())
}

func (r *Registry) self(ctx context.Context, action authz.Action) (*authn.Node, error) {
	if err := r.deps.Authorize(ctx, action); err != nil {
		return nil, err
	}
	self := authn.NodeFrom(ctx)
	if self == nil {
		return nil, api.Forbidden("only node agents may call this")
	}
	return self, nil
}

func (r *Registry) resolve(ctx context.Context, ref string) (*v1.Node, error) {
	var (
		node *v1.Node
		err  error
	)
	if v1.IsNodeID(ref) {
		node, err = r.deps.Store.GetNode(ctx, ref)
	} else {
		node, err = r.deps.Store.GetNodeByName(ctx, ref)
	}
	if err != nil {
		return nil, registry.StorageError(err, "node", ref)
	}
	return node, nil
}

func issueAPIKey(ctx context.Context, store storage.Interface, nodeID string, now time.Time) (*v1.NodeAPIKey, error) {
	plain, err := token.Generate()
	if err != nil {
		return nil, api.Internal("generate api key: %v", err)
	}
	expiresAt := now.Add(token.NodeAPIKeyTTL)
	if err := store.SetNodeAPIKey(ctx, nodeID, token.Hash(plain), expiresAt); err != nil {
		return nil, registry.StorageError(err, "node", nodeID)
	}
	return &v1.NodeAPIKey{NodeAPIKey: plain, APIKeyExpiresAt: expiresAt}, nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
