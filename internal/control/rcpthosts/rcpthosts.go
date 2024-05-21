package rcpthosts

import (
	"os"
	"strings"

	"qmail-smtpd/internal/constmap"
	"qmail-smtpd/internal/control"
)

var flagrh int
var maprh constmap.Constmap
var fdmrh *os.File // TODO

func Init() int {
	var rh []string

	rh, flagrh = control.ReadFile("control/rcpthosts", false)
	if flagrh != 1 {
		return flagrh
	}

	for i := range rh {
		rh[i] = strings.ToLower(rh[i])
	}

	maprh = constmap.New(rh)

	// TODO:
	// fdmrh = open_read("control/morercpthosts.cdb");
	// if (fdmrh == -1) if (errno != error_noent) return flagrh = -1;

	return 0
}

func Allowed(buf string) bool {
	if flagrh != 1 {
		return true
	}

	j := strings.IndexByte(buf, '@')
	if j == -1 {
		/* presumably envnoathost is acceptable */
		return true
	}

	j++
	buf = strings.ToLower(buf[j:])

	for j := range buf {
		if j == 0 || buf[j] == '.' {
			if maprh.Contains(buf[j:]) {
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
