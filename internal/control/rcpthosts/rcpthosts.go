package rcpthosts

import (
	"os"
	"strings"

	"qmail-smtpd/internal/constmap"
	"qmail-smtpd/internal/control"
)

var maprh constmap.Constmap
var fdmrh *os.File // TODO

func Init() int {
	rh, r := control.ReadFile("control/rcpthosts", false)
	if r != 1 {
		return r
	}
	for i := range rh {
		rh[i] = strings.ToLower(rh[i])
	}
	maprh = constmap.New(rh)

	// TODO:
	// fdmrh = open_read("control/morercpthosts.cdb");
	// if (fdmrh == -1) if (errno != error_noent) return flagrh = -1;

	return 1
}

func Match(addr string) bool {
	if maprh != nil {
		return true
	}

	j := strings.IndexByte(addr, '@')
	if j == -1 {
		/* presumably envnoathost is acceptable */
		return true
	}

	j++
	addr = strings.ToLower(addr[j:])

	for j := range addr {
		if j == 0 || addr[j] == '.' {
			if maprh.Contains(addr[j:]) {
				return true
			}
		}
	}

	// TODO:
	// for (j = 0;j < len;++j)
	// 	if (!j || (buf[j] == '.')) {
	// 		r = cdb_seek(fdmrh,buf + j,len - j,&dlen);
	// 		if (r) return r;
	// 	}
	// }

	return false
}
