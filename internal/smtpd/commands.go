package smtpd

import (
	"strings"
)

type handlerFunc = func(ss *session, arg string) error

type command struct {
	handler   handlerFunc
	needFlush bool
}

type commandTable map[string]command

const unimpl = "*unimpl*"

func newCommandTable(d *Server) commandTable {
	return map[string]command{
		"rcpt":     {d.smtp_rcpt, false},
		"mail":     {d.smtp_mail, false},
		"data":     {d.smtp_data, true},
		"auth":     {d.smtp_auth, true},
		"quit":     {d.smtp_quit, true},
		"helo":     {d.smtp_helo, true},
		"ehlo":     {d.smtp_ehlo, true},
		"rset":     {d.smtp_rset, false},
		"help":     {d.smtp_help, true},
		"starttls": {d.smtp_tls, false},
		"noop":     {d.err_noop, true},   // WTF: почему err_noop, а не smtp_noop?
		"vrfy":     {d.err_vrfy, true},   // WTF: аналогично?
		unimpl:     {d.err_unimpl, true}, // WTF: аналогично?
	}
}

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
