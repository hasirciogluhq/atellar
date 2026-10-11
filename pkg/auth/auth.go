// Package auth wires authentication for the API server: it maps bearer
// tokens to node or admin principals.
package auth

import (
	"context"
	"errors"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/auth/authz"
	"github.com/hasirciogluhq/atellar/pkg/auth/token"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

// NodeLookup finds a node by the SHA-256 hash of its API key and returns
// storage.ErrNotFound when no live key matches.
type NodeLookup func(ctx context.Context, hash string) (*v1.Node, error)

// TokenAuthenticator accepts the admin token and node API keys.
type TokenAuthenticator struct {
	adminToken string
	nodes      NodeLookup
}

// NewTokenAuthenticator returns an authenticator. An empty adminToken
// disables admin access by token; it never makes anonymous callers admin.
func NewTokenAuthenticator(adminToken string, nodes NodeLookup) *TokenAuthenticator {
	return &TokenAuthenticator{adminToken: adminToken, nodes: nodes}
}

func (a *TokenAuthenticator) Authenticate(ctx context.Context, plain string) (*authn.Principal, error) {
	if plain == "" {
		return nil, nil
	}
	if a.adminToken != "" && token.Equal(plain, a.adminToken) {
		return adminPrincipal(), nil
	}

	node, err := a.nodes(ctx, token.Hash(plain))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, authn.ErrInvalidCredential
	}
	if err != nil {
		return nil, err
	}
	// Eviction revokes the node's credential.
	if node.Status == v1.NodeStatusEvicted {
		return nil, authn.ErrInvalidCredential
	}
	return &authn.Principal{
		Subject: authn.SubjectNode,
		Node:    &authn.Node{ID: node.ID, Name: node.Name},
		Scopes:  authz.NodeAgentScopes(),
	}, nil
}

func adminPrincipal() *authn.Principal {
	return &authn.Principal{Subject: authn.SubjectAdmin, Scopes: authz.AdminScopes()}
}
