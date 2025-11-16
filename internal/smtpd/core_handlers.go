package smtpd

import (
	"cmp"
	"strconv"
	"strings"
)

func (d *Server) smtp_greet(ss *session, code string) error {
	_ = ss.out(code)
	return ss.out(d.cfg.Greeting)
}

func (d *Server) smtp_help(ss *session, _ string) error {
	return ss.out("214 qmail home page: http://pobox.com/~djb/qmail.html\r\n")
}

func (d *Server) smtp_quit(ss *session, _ string) error {
	_ = d.smtp_greet(ss, "221 ")
	_ = ss.out("\r\n")
	return cmp.Or(ss.Flush(), ErrClientQuit)
}

func (d *Server) dohelo(ss *session, arg string) {
	ss.helohost = arg
	if !strings.EqualFold(ss.remoteHost, ss.helohost) {
		ss.fakehelo = ss.helohost
	}
}

func (d *Server) smtp_helo(ss *session, arg string) error {
	ss.seenmail = false
	d.dohelo(ss, arg)
	_ = d.smtp_greet(ss, "250 ")
	return ss.out("\r\n")
}

func (d *Server) smtp_ehlo(ss *session, arg string) error {
	ss.seenmail = false
	d.dohelo(ss, arg)
	_ = d.smtp_greet(ss, "250-")
	if d.cfg.Auth != nil && !ss.authorized {
		if ss.tlsEnabled {
			_ = ss.out("\r\n250-AUTH LOGIN CRAM-MD5 PLAIN")
			_ = ss.out("\r\n250-AUTH=LOGIN CRAM-MD5 PLAIN") // WTF? =
		} else {
			_ = ss.out("\r\n250-AUTH CRAM-MD5")
			_ = ss.out("\r\n250-AUTH=CRAM-MD5") // WTF? =
		}
	}
	if d.cfg.Databytes > 0 {
		_ = ss.out("\r\n250-SIZE ")
		_ = ss.out(strconv.Itoa(d.cfg.Databytes))
	}
	if d.cfg.TLSConfig != nil && !ss.tlsEnabled {
		_ = ss.out("\r\n250-STARTTLS")
	}
	_ = ss.out("\r\n250-PIPELINING")
	return ss.out("\r\n250 8BITMIME\r\n")
}

func (d *Server) smtp_rset(ss *session, args string) error {
	ss.seenmail = false
	return ss.out("250 flushed\r\n")
}

func (d *Server) smtp_mail(ss *session, arg string) error {
	addr, ok := addrparse(arg)
	if !ok {
		return d.err_syntax(ss)
	}
	if d.cfg.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.cfg.LocalIPHost, d.cfg.IPMe)
	}
	// TODO: check SIZE parameter
	ss.flagbarf = d.cfg.BadMailFrom != nil && d.cfg.BadMailFrom.Match(addr)
	ss.seenmail = true
	ss.rcptto = ss.rcptto[:0]
	ss.mailfrom = addr
	return ss.out("250 ok\r\n")
}

func (d *Server) smtp_rcpt(ss *session, arg string) error {
	if !ss.seenmail {
		return d.err_wantmail(ss)
	}
	addr, ok := addrparse(arg)
	if !ok {
		return d.err_syntax(ss)
	}
	if d.cfg.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.cfg.LocalIPHost, d.cfg.IPMe)
	}
	if ss.flagbarf {
		return d.err_bmf(ss)
	}
	if ss.relayclientok {
		addr += ss.relayclient
	} else {
		if d.cfg.RcptHosts != nil && !d.cfg.RcptHosts.Match(addr) {
			return d.err_nogateway(ss)
		}
		// Дополнительная проверка: если домен в mbxhosts, проверить существование ящика
		if d.cfg.MbxHosts != nil && !d.cfg.MbxHosts.Match(addr) {
			return ss.out("553 mailbox does not exist (#5.1.1)\r\n")
		}
	}
	ss.rcptto = append(ss.rcptto, addr)
	return ss.out("250 ok\r\n")
}
