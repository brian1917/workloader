package wkldimport

import (
	"fmt"
	"net"
	"strings"

	"github.com/brian1917/illumioapi"
)

// userInputConvert takes an ip address in the format of eth0:192.168.20.21 and returns an illumio interface struct
func userInputConvert(ip string) (illumioapi.Interface, error) {

	x := strings.Split(ip, ":")

	// If the length is 1, we know it's IPv4 with no interface name
	if len(x) == 1 {
		nic, err := ipCheck(ip)
		nic.Name = "umwl"
		return nic, err
	}

	// If the length is 2, we know it's an IPv4 address with an interface name
	if len(x) == 2 {
		nic, err := ipCheck(x[1])
		nic.Name = x[0]
		return nic, err
	}

	// If the length is greater than 2, it's an IPv6 address. Check with no name. If err is nil, return it
	nic, err := ipCheck(ip)
	if err == nil {
		nic.Name = "umwl"
		return nic, err
	}

	// Check with name
	nic, err = ipCheck(strings.Join(x[1:], ":"))
	if err == nil {
		nic.Name = x[0]
		return nic, err
	}

	return illumioapi.Interface{}, fmt.Errorf("%s is an invalid ip format", ip)
}

func ipCheck(ip string) (illumioapi.Interface, error) {
	if strings.Contains(ip, "/") {
		ipAddress, ipNet, err := net.ParseCIDR(ip)
		if err == nil {
			cidr, _ := ipNet.Mask.Size()
			return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: &cidr}, nil
		}
	}

	ipAddress := net.ParseIP(ip)
	if ipAddress != nil {
		return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: nil}, nil
	}

	return illumioapi.Interface{}, fmt.Errorf("invalid IP address")
}

// publicIPIsValid validates the ip string is either a valid CIDR or IP address
func publicIPIsValid(ip string) bool {

	if ip == "" {
		return true
	}

	if strings.Contains(ip, "/") {
		_, _, err := net.ParseCIDR(ip)
		return err == nil
	}

	i := net.ParseIP(ip)
	return i != nil

}
