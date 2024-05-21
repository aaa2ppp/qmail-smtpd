package ipme

import (
	"net"

	"qmail-smtpd/internal/scan"
)

var ipmeok bool
var ipme []scan.IPAddress

func Init() bool {
	if ipmeok {
		return true
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		n, ip := scan.ScanIP(addr.String())
		if n > 0 {
			ipme = append(ipme, ip)
		}
	}
	ipmeok = true
	return true
}

func Is(ip scan.IPAddress) bool {
	for i := range ipme {
		if ipme[i] == ip {
			return true
		}
	}
	return false
}
