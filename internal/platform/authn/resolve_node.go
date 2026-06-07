package authn

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/metadata"
)

const metadataNodeID = "x-node-id"

var ErrNodeIDRequired = errors.New("node_id is required")

// ResolveNodeID returns the node ID from principal.Node when set, otherwise the first
// non-empty fallback (request field, query param, path param, etc.).
func ResolveNodeID(principal *Principal, fallbacks ...string) (string, error) {
	if id := nodeIDFromPrincipal(principal); id != "" {
		return id, nil
	}
	for _, fb := range fallbacks {
		if id := strings.TrimSpace(fb); id != "" {
			return id, nil
		}
	}
	return "", ErrNodeIDRequired
}

// ResolveNodeIDFromContext resolves node ID from context principal, gRPC metadata, then fallbacks.
func ResolveNodeIDFromContext(ctx context.Context, fallbacks ...string) (string, error) {
	principal, _ := PrincipalFromContext(ctx)
	if id := nodeIDFromPrincipal(principal); id != "" {
		return id, nil
	}
	if id := nodeIDFromGRPCMetadata(ctx); id != "" {
		return id, nil
	}
	return ResolveNodeID(principal, fallbacks...)
}

func nodeIDFromPrincipal(principal *Principal) string {
	if principal == nil || principal.Node == nil {
		return ""
	}
	return strings.TrimSpace(principal.Node.ID)
}

func nodeIDFromGRPCMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, key := range []string{metadataNodeID, "node-id", "node_id"} {
		values := md.Get(key)
		if len(values) == 0 {
			continue
		}
		if id := strings.TrimSpace(values[0]); id != "" {
			return id
		}
	}
	return ""
}
