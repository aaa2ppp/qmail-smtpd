package main

import (
	"os"
	"strings"
)

var flagrh int
var maprh = tConstmap{}
var fdmrh *os.File

func rcpthosts_init() int {
	var rh []string

	rh, flagrh = control_readfile("control/rcpthosts", false)
	if flagrh != 1 {
		return flagrh
	}

	constmap_init(maprh, rh)

	// TODO:
	// fdmrh = open_read("control/morercpthosts.cdb");
	// if (fdmrh == -1) if (errno != error_noent) return flagrh = -1;

	return 0
}

func rcpthosts(buf string) int {
	if flagrh != 1 {
		return 1
	}

	j := strings.IndexByte(buf, '@')
	if j == -1 {
		/* presumably envnoathost is acceptable */
		return 1
	}

	j++
	buf = strings.ToLower(buf[j:])

	for j := range buf {
		if j == 0 || buf[j] == '.' {
			if constmap(maprh, buf[j:]) {
				return 1
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

	return 0
}
