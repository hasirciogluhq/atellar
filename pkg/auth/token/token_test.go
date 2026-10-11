package token

import (
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	a, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Generate()
	if len(a) != 64 || a == b {
		t.Fatalf("Generate returned %q and %q", a, b)
	}
}

func TestHashAndEqual(t *testing.T) {
	if Hash("x") != Hash("x") || Hash("x") == Hash("y") || len(Hash("x")) != 64 {
		t.Fatal("Hash is not a stable SHA-256 hex digest")
	}
	if !Equal("abc", "abc") || Equal("abc", "abd") || Equal("abc", "ab") {
		t.Fatal("Equal is wrong")
	}
}

func TestShouldRenew(t *testing.T) {
	cases := map[time.Duration]bool{
		NodeAPIKeyTTL:                     false,
		NodeAPIKeyRenewBefore + time.Hour: false,
		NodeAPIKeyRenewBefore - time.Hour: true,
		-time.Hour:                        true,
	}
	for left, want := range cases {
		if got := ShouldRenewNodeAPIKey(time.Now().Add(left)); got != want {
			t.Errorf("ShouldRenew(%s left) = %v, want %v", left, got, want)
		}
	}
}
