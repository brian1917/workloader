package iplimport

import (
	"net"
	"strings"
)

func ValidateIplistEntry(entry string) bool {
	// Process cidr
	if strings.Contains(entry, "/") {
		_, _, err := net.ParseCIDR(entry)
		if err != nil {
			return false
		} else {
			return true
		}
	}

	// Process non cidr
	for _, i := range strings.Split(entry, "-") {
		if ipAddress := net.ParseIP(i); ipAddress == nil {
			return false
		}
	}
	return true
}
