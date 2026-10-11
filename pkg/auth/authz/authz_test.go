package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
)

func TestAuthorize(t *testing.T) {
	a := New(nil)
	node := &authn.Principal{Subject: authn.SubjectNode, Scopes: NodeAgentScopes()}
	admin := &authn.Principal{Subject: authn.SubjectAdmin, Scopes: AdminScopes()}

	cases := []struct {
		name      string
		principal *authn.Principal
		action    Action
		want      error
	}{
		{"anonymous", nil, ActionClusterReadNodes, ErrUnauthenticated},
		{"node heartbeat", node, ActionAgentHeartbeat, nil},
		{"node reads nodes for overlay", node, ActionClusterReadNodes, nil},
		{"node cannot deploy", node, ActionClusterDeployContainer, ErrForbidden},
		{"node cannot mint join tokens", node, ActionClusterManageJoinTokens, ErrForbidden},
		{"admin deploys", admin, ActionClusterDeployContainer, nil},
		{"admin cannot heartbeat", admin, ActionAgentHeartbeat, ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.principal != nil {
				ctx = authn.WithPrincipal(ctx, tc.principal)
			}
			err := a.Authorize(ctx, tc.action)
			if tc.want == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
