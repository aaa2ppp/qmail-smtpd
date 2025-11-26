package smtpd

import (
	"log/slog"
	"net"
)

type IPMe struct {
	ips []net.IP
	err error
}

func (p *IPMe) init() {
	if p.err != nil {
		return
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		// что-то ненормальное
		p.err = err
		slog.Error("InterfaceAddrs", "error", err)
		return
	}

	var ips []net.IP
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			ips = append(ips, ipNet.IP)
		}
	}

	p.ips = ips
}

func (p *IPMe) Contains(ip net.IP) bool {
	if p.ips == nil {
		p.init()
	}

	for _, ipme := range p.ips {
		if ip.Equal(ipme) {
			return true
		}
	}

	return false
}
