// Package authn identifies the caller of an API request. Transports turn a
// bearer token into a Principal and store it in the request context.
package authn

import (
	"context"
	"errors"
	"strings"
)

const (
	HeaderAuthorization = "Authorization"
	SchemeBearer        = "Bearer"
)

var ErrInvalidCredential = errors.New("invalid credential")

type SubjectType string

const (
	SubjectNode  SubjectType = "node"
	SubjectAdmin SubjectType = "admin"
)

type Node struct {
	ID   string
	Name string
}

// Principal is an authenticated caller.
type Principal struct {
	Subject SubjectType
	Node    *Node // set for SubjectNode
	Scopes  []string
}

// Authenticator resolves a bearer token. An empty token yields a nil
// principal (anonymous) unless the implementation decides otherwise.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*Principal, error)
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, principal)
}

// PrincipalFrom returns the caller, or nil for anonymous requests.
func PrincipalFrom(ctx context.Context) *Principal {
	principal, _ := ctx.Value(ctxKey{}).(*Principal)
	return principal
}

// NodeFrom returns the calling node, or nil when the caller is not a node.
func NodeFrom(ctx context.Context) *Node {
	principal := PrincipalFrom(ctx)
	if principal == nil || principal.Subject != SubjectNode {
		return nil
	}
	return principal.Node
}

// ParseBearer extracts the token from an Authorization header value.
// An empty header returns "" and no error.
func ParseBearer(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", nil
	}
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, SchemeBearer) {
		return "", ErrInvalidCredential
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrInvalidCredential
	}
	return token, nil
}

// Bearer formats a token as an Authorization header value.
func Bearer(token string) string {
	return SchemeBearer + " " + token
}
