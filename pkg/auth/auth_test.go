package auth

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/auth/token"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

// TestAuthenticate covers mode (open or closed) x presented credential.
func TestAuthenticate(t *testing.T) {
	nodes := map[string]v1.Node{
		token.Hash("ready-key"):   {ID: "node_ready", Name: "ready", Status: v1.NodeStatusReady},
		token.Hash("evicted-key"): {ID: "node_gone", Name: "gone", Status: v1.NodeStatusEvicted},
	}
	lookup := func(_ context.Context, hash string) (*v1.Node, error) {
		if hash == token.Hash("broken-key") {
			return nil, errors.New("database down")
		}
		n, ok := nodes[hash]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return &n, nil
	}

	type want struct {
		subject authn.SubjectType // "" means anonymous
		err     bool
	}
	cases := []struct {
		token        string
		closed, open want
	}{
		{"", want{}, want{subject: authn.SubjectAdmin}},
		{"admin-secret", want{subject: authn.SubjectAdmin}, want{err: true}},
		{"ready-key", want{subject: authn.SubjectNode}, want{subject: authn.SubjectNode}},
		{"evicted-key", want{err: true}, want{err: true}},
		{"unknown", want{err: true}, want{err: true}},
		{"broken-key", want{err: true}, want{err: true}},
	}
	for _, tc := range cases {
		for _, mode := range []struct {
			name  string
			admin string
			want  want
		}{{"closed", "admin-secret", tc.closed}, {"open", "", tc.open}} {
			a := NewTokenAuthenticator(mode.admin, lookup)
			p, err := a.Authenticate(context.Background(), tc.token)
			if (err != nil) != mode.want.err {
				t.Errorf("%s/%q: err = %v, want error %v", mode.name, tc.token, err, mode.want.err)
				continue
			}
			var subject authn.SubjectType
			if p != nil {
				subject = p.Subject
			}
			if subject != mode.want.subject {
				t.Errorf("%s/%q: subject = %q, want %q", mode.name, tc.token, subject, mode.want.subject)
			}
		}
	}
}
