package ipam

import "testing"

func TestAllocateNextSkipsUsedSubnets(t *testing.T) {
	a, err := NewAllocator("10.0.0.0/16", 24)
	if err != nil {
		t.Fatal(err)
	}

	got, err := a.AllocateNext([]string{"10.0.0.0/24", "10.0.1.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if got.SubnetCIDR != "10.0.2.0/24" {
		t.Fatalf("subnet = %s, want 10.0.2.0/24", got.SubnetCIDR)
	}
	if got.NodeIP.String() != "10.0.2.1" {
		t.Fatalf("node ip = %s, want 10.0.2.1", got.NodeIP)
	}
	// .0 network, .1 node, .255 broadcast are excluded from a /24 pool.
	if len(got.PoolIPs) != 253 || got.PoolIPs[0].String() != "10.0.2.2" {
		t.Fatalf("pool = %d ips starting %s, want 253 starting 10.0.2.2", len(got.PoolIPs), got.PoolIPs[0])
	}
}

func TestAllocatePrefersReclaimable(t *testing.T) {
	a, err := NewAllocator("10.0.0.0/16", 24)
	if err != nil {
		t.Fatal(err)
	}

	got, err := a.Allocate([]ReclaimableSubnet{{SourceNodeID: "node_old", SubnetCIDR: "10.0.7.0/24"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceNodeID != "node_old" || got.Allocation.SubnetCIDR != "10.0.7.0/24" {
		t.Fatalf("got %+v, want reclaimed 10.0.7.0/24 from node_old", got)
	}
}

func TestAllocateNextExhausted(t *testing.T) {
	a, err := NewAllocator("10.0.0.0/23", 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AllocateNext([]string{"10.0.0.0/24", "10.0.1.0/24"}); err != ErrNoSubnetsAvailable {
		t.Fatalf("err = %v, want ErrNoSubnetsAvailable", err)
	}
}
