package authn

import (
	"context"
	"testing"
)

func TestParseBearer(t *testing.T) {
	cases := []struct {
		header, token string
		ok            bool
	}{
		{"", "", true},
		{"   ", "", true},
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"Bearer   abc  ", "abc", true},
		{"Bearer", "", false},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"abc", "", false},
	}
	for _, tc := range cases {
		got, err := ParseBearer(tc.header)
		if (err == nil) != tc.ok || got != tc.token {
			t.Errorf("ParseBearer(%q) = %q, %v", tc.header, got, err)
		}
	}
	if Bearer("x") != "Bearer x" {
		t.Error("Bearer format")
	}
}

func TestPrincipalContext(t *testing.T) {
	ctx := context.Background()
	if PrincipalFrom(ctx) != nil || NodeFrom(ctx) != nil {
		t.Fatal("empty context has a principal")
	}
	admin := WithPrincipal(ctx, &Principal{Subject: SubjectAdmin})
	if NodeFrom(admin) != nil || PrincipalFrom(admin).Subject != SubjectAdmin {
		t.Fatal("admin context")
	}
	node := WithPrincipal(ctx, &Principal{Subject: SubjectNode, Node: &Node{ID: "node_a"}})
	if n := NodeFrom(node); n == nil || n.ID != "node_a" {
		t.Fatal("node context")
	}
}
