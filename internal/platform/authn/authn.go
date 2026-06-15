package authn

import (
	"context"
	"errors"
	"strings"
)

const (
	HeaderAuthorization = "Authorization"

	SchemeBearer = "Bearer"
)

var (
	ErrMissingCredential  = errors.New("missing credential")
	ErrInvalidCredential  = errors.New("invalid credential")
	ErrUnsupportedSubject = errors.New("unsupported subject")
)

type SubjectType string

const SubjectTypeNode SubjectType = "node"

type CredentialType string

const CredentialTypeNodeAPIKey CredentialType = "node_api_key"

type Node struct {
	ID   string
	Name string
}

type Principal struct {
	SubjectType SubjectType
	Node        *Node
	Scopes      []string
}

type Credential struct {
	Type  CredentialType
	Value string
}

type Authenticator interface {
	Authenticate(ctx context.Context, credential Credential) (*Principal, error)
}

type ctxKey struct{}

func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	principal, ok := ctx.Value(ctxKey{}).(*Principal)
	return principal, ok
}

func MustNodeFromContext(ctx context.Context) (*Node, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || principal == nil || principal.SubjectType != SubjectTypeNode || principal.Node == nil {
		return nil, ErrInvalidCredential
	}

	return principal.Node, nil
}

func ParseAuthorizationHeader(headerValue string) (Credential, error) {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return Credential{}, ErrMissingCredential
	}

	parts := strings.SplitN(headerValue, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], SchemeBearer) {
		return Credential{}, ErrInvalidCredential
	}

	apiKey := strings.TrimSpace(parts[1])
	if apiKey == "" {
		return Credential{}, ErrInvalidCredential
	}

	return Credential{
		Type:  CredentialTypeNodeAPIKey,
		Value: apiKey,
	}, nil
}
