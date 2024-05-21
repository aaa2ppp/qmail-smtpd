package badmailfrom

import (
	"strings"

	"qmail-smtpd/internal/constmap"
	"qmail-smtpd/internal/control"
)

var bmfok bool
var mapbmf constmap.Constmap

func Init() int {
	if ss, r := control.ReadFile("control/badmailfrom", false); r == -1 {
		return -1
	} else if r == 1 {
		mapbmf = constmap.New(ss)
		bmfok = true
	}
	return 1
}

func Allowed(addr string) bool {
	if !bmfok {
		return true
	}
	if mapbmf.Contains(addr) {
		return false
	}
	if j := strings.IndexByte(addr, '@'); j != -1 {
		if mapbmf.Contains(addr[j+1:]) {
			return false
		}
	}
	return true
}
