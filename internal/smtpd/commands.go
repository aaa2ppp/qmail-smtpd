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

func (d *Server) commandLoop(ss *session) error {
	for {
		line, err := ss.ReadLine()
		if err != nil {
			return err
		}

		name, arg := parseCmdLine(line)
		cmd, ok := d.cmdTable[name]
		if !ok {
			cmd = d.cmdTable[unimpl]
		}

		if err := cmd.handler(ss, arg); err != nil {
			return err
		}

		if !cmd.needFlush {
			continue
		}

		if err := ss.Flush(); err != nil {
			return err
		}
	}
}
