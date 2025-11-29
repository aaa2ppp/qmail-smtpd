package smtpd

import (
	"cmp"
	"errors"
	"os"
	"strconv"
	"strings"
)

var ErrClientQuit = errors.New("client quit")

func smtp_run(ss *session, commands commands) error {
	dohelo(ss, ss.remoteHosts)
	_ = smtp_greet(ss, "220 ")
	_ = ss.out(" ")
	_ = ss.out(ss.proto)
	_ = ss.out("\r\n")
	if err := ss.io.Flush(); err != nil {
		return err
	}

	err := commands.loop(ss) // always return error
	switch {
	case err == ErrClientQuit:
		return nil
	case errors.Is(err, os.ErrDeadlineExceeded):
		err_timeout(ss)
		ss.io.Flush()
	}

	return err
}

func smtp_greet(ss *session, code string) error {
	_ = ss.out(code)
	return ss.out(ss.greeting)
}

func smtp_help(ss *session, _ string) error {
	return ss.out("214 qmail home page: http://pobox.com/~djb/qmail.html\r\n")
}

func smtp_quit(ss *session, _ string) error {
	_ = smtp_greet(ss, "221 ")
	_ = ss.out("\r\n")
	return cmp.Or(ss.io.Flush(), ErrClientQuit)
}

func dohelo(ss *session, arg string) {
	ss.state = state{} // clear the state
	resetAuthorization(ss)
	ss.state.heloHost = arg
	ss.state.fakeHelo = !strings.EqualFold(ss.remoteHosts, ss.state.heloHost)
}

func smtp_helo(ss *session, arg string) error {
	dohelo(ss, arg)
	_ = smtp_greet(ss, "250 ")
	return ss.out("\r\n")
}

func smtp_ehlo(ss *session, arg string) error {
	dohelo(ss, arg)
	_ = smtp_greet(ss, "250-")

	if ss.auth != nil && !ss.state.authorized {
		if ss.tlsEnabled || ss.unsafeAuth {
			_ = ss.out("\r\n250-AUTH LOGIN CRAM-MD5 PLAIN")
			_ = ss.out("\r\n250-AUTH=LOGIN CRAM-MD5 PLAIN") // WTF? =
		} else {
			_ = ss.out("\r\n250-AUTH CRAM-MD5")
			_ = ss.out("\r\n250-AUTH=CRAM-MD5") // WTF? =
		}
	}

	if ss.databytes > 0 {
		_ = ss.out("\r\n250-SIZE ")
		_ = ss.out(strconv.Itoa(ss.databytes))
	}

	if ss.tlsConfig != nil && !ss.tlsEnabled {
		_ = ss.out("\r\n250-STARTTLS")
	}

	_ = ss.out("\r\n250-PIPELINING")
	return ss.out("\r\n250 8BITMIME\r\n")
}

func smtp_rset(ss *session, args string) error {
	ss.state.seenMail = false
	return ss.out("250 flushed\r\n")
}

func smtp_mail(ss *session, arg string) error {
	addr, ok := addrparse(arg)
	if !ok {
		return err_syntax(ss)
	}

	if ss.localIPHost != "" {
		addr = replaceLocalIP(addr, ss.localIPHost, ss.ipme)
	}

	allow, reason := mailValidate(ss, addr)

	if allow {
		// TODO: check SIZE parameter
	}

	if !allow {
		ss.state.flagbarf = true
		ss.state.bmfReason = reason
	} else {
		ss.state.flagbarf = false
		ss.state.bmfReason = ""
	}

	ss.state.seenMail = true
	ss.state.mailFrom = addr
	ss.state.rcptTo = ss.state.rcptTo[:0]

	return ss.out("250 ok\r\n")
}

func smtp_rcpt(ss *session, arg string) error {
	if !ss.state.seenMail {
		return err_wantmail(ss)
	}

	addr, ok := addrparse(arg)
	if !ok {
		return err_syntax(ss)
	}
	if ss.localIPHost != "" {
		addr = replaceLocalIP(addr, ss.localIPHost, ss.ipme)
	}

	if ss.state.flagbarf {
		_ = ss.out(ss.state.bmfReason)
		return ss.out("\r\n")
	}

	if allow, reason := rcptValidate(ss, addr); !allow {
		_ = ss.out(reason)
		return ss.out("\r\n")
	}

	if ss.state.relaySuffix != "" {
		addr += ss.state.relaySuffix
	}

	ss.state.rcptTo = append(ss.state.rcptTo, addr)
	return ss.out("250 ok\r\n")
}

func smtp_noop(ss *session, _ string) error {
	return ss.out("250 ok\r\n")
}

func smtp_vrfy(ss *session, _ string) error {
	return ss.out("252 send some mail, i'll try my best\r\n")
}

func smtp_unimpl(ss *session, _ string) error {
	return ss.out("502 unimplemented (#5.5.1)\r\n")
}
