package smtpd

import (
	"strings"
)

func parseCmdLine(line string) (name, arg string) {
	p := strings.IndexByte(line, ' ')
	if p == -1 {
		p = len(line)
	}
	return strings.ToLower(line[:p]), strings.TrimSpace(line[p:])
}

func (d *Smtpd) commands(ss *Session, c map[string]command) error {
	for {
		line, err := ss.getln()
		if err != nil {
			return err
		}

		name, arg := parseCmdLine(line)
		cmd, ok := c[name]
		if !ok {
			cmd = c[unimpl]
		}

		if err := cmd.handler(ss, arg); err != nil {
			return err
		}

		if !cmd.needFlush {
			continue
		}

		if err := ss.flush(); err != nil {
			return err
		}
	}
}
