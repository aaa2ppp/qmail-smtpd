package smtpd

import (
	"strings"
)

type command struct {
	name  string
	fun   func(arg string)
	flush func()
}

func (d *Smtpd) commands(c []command) error {
	if len(c) == 0 || c[len(c)-1].name != "" {
		panic("name of last command must be empty")
	}

	for {
		cmd := d.getln()
		i := strings.IndexByte(cmd, ' ')
		if i == -1 {
			i = len(cmd)
		}

		arg := cmd[i:]
		for len(arg) > 0 && arg[0] == ' ' {
			arg = arg[1:]
		}
		cmd = cmd[:i]

		{
			i := 0 // xxx
			for ; c[i].name != ""; i++ {
				if strings.EqualFold(c[i].name, cmd) {
					break
				}
			}
			c[i].fun(arg)
			if c[i].flush != nil {
				c[i].flush()
			}
		}
	}
}
