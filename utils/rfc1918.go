package utils

import "net"

// IsRFC1918 returns true if an IP address is in the RFC1918 range
func IsRFC1918(ipAddress string) bool {

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false
	}

	// Define the private IP ranges
	privateIPBlocks := []*net.IPNet{
		{
			IP:   net.IPv4(10, 0, 0, 0),
			Mask: net.CIDRMask(8, 32),
		},
		{
			IP:   net.IPv4(172, 16, 0, 0),
			Mask: net.CIDRMask(12, 32),
		},
		{
			IP:   net.IPv4(192, 168, 0, 0),
			Mask: net.CIDRMask(16, 32),
		},
	}

	// Check if the IP is within any of the private IP ranges
	for _, privateIPBlock := range privateIPBlocks {
		if privateIPBlock.Contains(ip) {
			return true
		}
	}

	return false
}
