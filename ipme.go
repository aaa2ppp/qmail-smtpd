package main

import "net"

var ipmeok bool
var ipme []ip_address

func ipme_is(ip ip_address) bool {
	for i := range ipme {
		if ipme[i] == ip {
			return true
		}
	}
	return false
}

func ipme_init() bool {
	if ipmeok {
		return true
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		n, ip := ip_scan(addr.String())
		if n > 0 {
			ipme = append(ipme, ip)
		}
	}
	ipmeok = true
	return true
}
