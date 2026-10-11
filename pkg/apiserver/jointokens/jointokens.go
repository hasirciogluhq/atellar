// Package jointokens implements api.JoinTokenInterface.
package jointokens

import (
	"context"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/registry"
	"github.com/hasirciogluhq/atellar/pkg/auth/authz"
	"github.com/hasirciogluhq/atellar/pkg/auth/token"
)

type Registry struct {
	deps *registry.Deps
}

var _ api.JoinTokenInterface = (*Registry)(nil)

func New(deps *registry.Deps) *Registry {
	return &Registry{deps: deps}
}

func (r *Registry) List(ctx context.Context) ([]v1.JoinToken, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterManageJoinTokens); err != nil {
		return nil, err
	}
	tokens, err := r.deps.Store.ListJoinTokens(ctx)
	return tokens, registry.StorageError(err, "join tokens", "")
}

// Create mints a token. The plain secret is returned once and only its hash
// is stored. Tokens are single use unless the request says otherwise.
func (r *Registry) Create(ctx context.Context, req v1.CreateJoinTokenRequest) (*v1.JoinToken, error) {
	if err := r.deps.Authorize(ctx, authz.ActionClusterManageJoinTokens); err != nil {
		return nil, err
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		return nil, api.BadRequest("expires_at is in the past")
	}
	singleUse := true
	if req.SingleUse != nil {
		singleUse = *req.SingleUse
	}

	plain, err := token.Generate()
	if err != nil {
		return nil, api.Internal("generate token: %v", err)
	}
	created, err := r.deps.Store.CreateJoinToken(ctx, token.Hash(plain), singleUse, req.ExpiresAt)
	if err != nil {
		return nil, registry.StorageError(err, "join token", "")
	}
	created.Token = plain
	return created, nil
}
