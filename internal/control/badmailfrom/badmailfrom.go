package badmailfrom

import (
	"strings"

	"qmail-smtpd/internal/constmap"
	"qmail-smtpd/internal/control"
)

var mapbmf constmap.Constmap

func Init() int {
	bmf, r := control.ReadFile("control/badmailfrom", false)
	if r != 1 {
		return r
	}
	for i := range bmf {
		bmf[i] = strings.ToLower(bmf[i])
	}
	mapbmf = constmap.New(bmf)
	return 1
}

func Match(addr string) bool {
	if mapbmf == nil {
		return false
	}
	addr = strings.ToLower(addr)
	if mapbmf.Contains(addr) {
		return true
	}
	if j := strings.IndexByte(addr, '@'); j != -1 {
		if mapbmf.Contains(addr[j+1:]) {
			return true
		}
	}
	return false
}
