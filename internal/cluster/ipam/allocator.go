package ipam

import (
	"errors"
	"fmt"
	"net"
)

const DefaultClusterCIDR = "10.0.0.0/8"

var ErrNoSubnetsAvailable = errors.New("no overlay subnets available in cluster CIDR")

type Allocation struct {
	SubnetCIDR string
	NodeIP     net.IP
	PoolIPs    []net.IP
}

type Allocator struct {
	clusterCIDR *net.IPNet
	prefixLen   int
}

func NewAllocator(clusterCIDR string, nodePrefixLen int) (*Allocator, error) {
	if clusterCIDR == "" {
		clusterCIDR = DefaultClusterCIDR
	}

	if nodePrefixLen == 0 {
		nodePrefixLen = 24
	}

	_, network, err := net.ParseCIDR(clusterCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster CIDR: %w", err)
	}

	ones, bits := network.Mask.Size()
	if nodePrefixLen < ones || nodePrefixLen > bits {
		return nil, fmt.Errorf("invalid node prefix length %d for cluster %s", nodePrefixLen, clusterCIDR)
	}

	return &Allocator{
		clusterCIDR: network,
		prefixLen:   nodePrefixLen,
	}, nil
}

func (a *Allocator) blockSize() uint32 {
	return uint32(1) << uint(32-a.prefixLen)
}

type ReclaimableSubnet struct {
	SourceNodeID string
	SubnetCIDR   string
}

type AllocationResult struct {
	Allocation   *Allocation
	SourceNodeID string // set when a reclaimed subnet is reused
}

func (a *Allocator) AllocationFromSubnet(subnetCIDR string) (*Allocation, error) {
	_, network, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet %q: %w", subnetCIDR, err)
	}

	nodeIP := nextHostIP(network.IP, network.Mask)
	if nodeIP == nil {
		return nil, fmt.Errorf("no host IP available in subnet %s", subnetCIDR)
	}

	poolIPs := hostPoolIPs(network, nodeIP)
	if len(poolIPs) == 0 {
		return nil, fmt.Errorf("no pool IPs available in subnet %s", subnetCIDR)
	}

	return &Allocation{
		SubnetCIDR: network.String(),
		NodeIP:     nodeIP,
		PoolIPs:    poolIPs,
	}, nil
}

// Allocate prefers reclaimable subnets from evicted nodes before carving a new block.
func (a *Allocator) Allocate(reclaimable []ReclaimableSubnet, activeSubnets []string) (*AllocationResult, error) {
	for _, candidate := range reclaimable {
		if candidate.SubnetCIDR == "" {
			continue
		}

		allocation, err := a.AllocationFromSubnet(candidate.SubnetCIDR)
		if err != nil {
			continue
		}

		return &AllocationResult{
			Allocation:   allocation,
			SourceNodeID: candidate.SourceNodeID,
		}, nil
	}

	allocation, err := a.AllocateNext(activeSubnets)
	if err != nil {
		return nil, err
	}

	return &AllocationResult{Allocation: allocation}, nil
}

func (a *Allocator) AllocateNext(allocatedSubnets []string) (*Allocation, error) {
	used := make([]*net.IPNet, 0, len(allocatedSubnets))
	for _, cidr := range allocatedSubnets {
		if cidr == "" {
			continue
		}

		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid allocated subnet %q: %w", cidr, err)
		}

		used = append(used, network)
	}

	step := a.blockSize()
	clusterIP := a.clusterCIDR.IP.To4()
	if clusterIP == nil {
		return nil, errors.New("only IPv4 cluster CIDR is supported")
	}

	base := ipToUint32(clusterIP) & mask32(a.clusterCIDR.Mask)
	clusterEnd := base | ^mask32(a.clusterCIDR.Mask)

	for offset := uint32(0); ; offset += step {
		candidate := base + offset
		if candidate > clusterEnd {
			return nil, ErrNoSubnetsAvailable
		}

		subnetIP := uint32ToIP(candidate)
		subnet := &net.IPNet{
			IP:   subnetIP,
			Mask: net.CIDRMask(a.prefixLen, 32),
		}

		if overlapsAny(subnet, used) {
			continue
		}

		nodeIP := nextHostIP(subnetIP, subnet.Mask)
		if nodeIP == nil {
			continue
		}

		poolIPs := hostPoolIPs(subnet, nodeIP)
		if len(poolIPs) == 0 {
			continue
		}

		return &Allocation{
			SubnetCIDR: subnet.String(),
			NodeIP:     nodeIP,
			PoolIPs:    poolIPs,
		}, nil
	}
}

func overlapsAny(candidate *net.IPNet, used []*net.IPNet) bool {
	for _, existing := range used {
		if candidate.Contains(existing.IP) || existing.Contains(candidate.IP) {
			return true
		}
	}

	return false
}

func nextHostIP(networkIP net.IP, mask net.IPMask) net.IP {
	ip := ipToUint32(networkIP.To4())
	end := ip | ^mask32(mask)

	for candidate := ip + 1; candidate < end; candidate++ {
		return uint32ToIP(candidate)
	}

	return nil
}

func hostPoolIPs(subnet *net.IPNet, nodeIP net.IP) []net.IP {
	start := ipToUint32(subnet.IP.To4())
	end := start | ^mask32(subnet.Mask)
	node := ipToUint32(nodeIP.To4())

	pool := make([]net.IP, 0)
	for candidate := start + 1; candidate < end; candidate++ {
		if candidate == node {
			continue
		}

		pool = append(pool, uint32ToIP(candidate))
	}

	return pool
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(value uint32) net.IP {
	return net.IPv4(byte(value>>24), byte(value>>16), byte(value>>8), byte(value))
}

func mask32(mask net.IPMask) uint32 {
	if len(mask) == 16 {
		return uint32(mask[12])<<24 | uint32(mask[13])<<16 | uint32(mask[14])<<8 | uint32(mask[15])
	}

	return uint32(mask[0])<<24 | uint32(mask[1])<<16 | uint32(mask[2])<<8 | uint32(mask[3])
}
