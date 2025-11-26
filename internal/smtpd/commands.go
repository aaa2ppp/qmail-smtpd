package smtpd

import (
	"strings"
)

type commandFunc = func(ss *session, arg string) error

type command struct {
	name      string
	handler   commandFunc
	needFlush bool
}

type commands []command

func newSMTPCommands() commands {
	return []command{
		{"rcpt", smtp_rcpt, false},
		{"mail", smtp_mail, false},
		{"data", smtp_data, true},
		{"auth", smtp_auth, true},
		{"quit", smtp_quit, true},
		{"helo", smtp_helo, true},
		{"ehlo", smtp_ehlo, true},
		{"rset", smtp_rset, false},
		{"help", smtp_help, true},
		{"starttls", smtp_starttls, false},
		{"noop", smtp_noop, true}, // WTF: почему err_noop, а не smtp_noop?
		{"vrfy", smtp_vrfy, true}, // WTF: аналогично?
		{"", smtp_unimpl, true},   // default; WTF: аналогично?
	}
}

func parseCmdLine(line string) (name, arg string) {
	p := strings.IndexByte(line, ' ')
	if p == -1 {
		p = len(line)
	}
	return line[:p], strings.TrimSpace(line[p:])
}

func (c commands) get(name string) command {
	i := 0
	for ; i < len(c)-1 && !strings.EqualFold(c[i].name, name); i++ {
	}
	return c[i]
}

func (c commands) loop(ss *session) error {
	for {
		line, err := ss.io.ReadLine()
		if err != nil {
			return err
		}

		name, arg := parseCmdLine(line)
		cmd := c.get(name)

		if err := cmd.handler(ss, arg); err != nil {
			return err
		}

		if !cmd.needFlush {
			continue
		}

		if err := ss.io.Flush(); err != nil {
			return err
		}
	}
}
