//go:build linux

package netns

import "testing"

func TestVethPairNames(t *testing.T) {
	seen := map[string]bool{}
	for _, id := range []string{"ctr_1", "ctr_2", "ctr_0123456789abcdef0123456789abcdef"} {
		host, peer := vethPairNames(id)
		if len(host) > maxIfaceNameLen || len(peer) > maxIfaceNameLen || host == peer {
			t.Fatalf("%s: %s %s", id, host, peer)
		}
		if seen[host] || seen[peer] {
			t.Fatalf("%s: name collision", id)
		}
		seen[host], seen[peer] = true, true
		if h2, p2 := vethPairNames(id); h2 != host || p2 != peer {
			t.Fatal("names are not deterministic")
		}
	}
}

func TestSubnetPrefixLen(t *testing.T) {
	cases := map[string]int{"10.0.1.0/24": 24, "10.0.0.0/16": 16, "": 32, "nope": 32}
	for in, want := range cases {
		if got := subnetPrefixLen(in); got != want {
			t.Errorf("subnetPrefixLen(%q) = %d, want %d", in, got, want)
		}
	}
}
