package mbxhosts

import (
	"net"
	"strings"

	"qmail-smtpd/internal/todo/constmap"
	"qmail-smtpd/internal/todo/control"
)

var (
	hostMap constmap.Constmap
	at      string
)

func Init() int {
	hosts, r := control.ReadFile("control/mbxhosts", false)
	if r != 1 {
		return r
	}
	zone, r := control.Rldef("control/mbxzone", false, "mbx")
	if r == -1 {
		return -1
	}
	for i := range hosts {
		hosts[i] = strings.ToLower(hosts[i])
	}
	hostMap = constmap.New(hosts)
	at = "." + zone + "."
	return 1
}

type address struct {
	Mailbox string
	Domain  string
}

func parseAddress(addr string) (address, bool) {
	j := strings.IndexByte(addr, '@')
	if j == -1 {
		return address{}, false
	}
	return address{
		Mailbox: addr[:j],
		Domain:  strings.ToLower(addr[j+1:]),
	}, true
}

func normalizeAddress(addr string) (address, bool) {
	if parsed, ok := parseAddress(addr); ok {
		return parsed, true
	}
	if !control.MeOk() {
		return address{}, false
	}
	return address{
		Mailbox: addr,
		Domain:  control.Me(),
	}, true
}

func Match(addr string) bool {
	if hostMap == nil {
		return true
	}

	parsed, ok := normalizeAddress(addr)
	if !ok {
		return true
	}

	if !hostMap.Contains(parsed.Domain) {
		return true
	}

	dnsName := parsed.Mailbox + at + parsed.Domain
	_, err := net.ResolveIPAddr("ip", dnsName)
	return err == nil
}
