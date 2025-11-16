package smtpd

import "time"

func issafe(ch byte) bool {
	switch {
	case ch == '.':
		return true
	case ch == '@':
		return true
	case ch == '%':
		return true
	case ch == '+':
		return true
	case ch == '/':
		return true
	case ch == '=':
		return true
	case ch == ':':
		return true
	case ch == '-':
		return true
	case (ch >= 'a') && (ch <= 'z'):
		return true
	case (ch >= 'A') && (ch <= 'Z'):
		return true
	case (ch >= '0') && (ch <= '9'):
		return true
	}
	return false
}

func safeput(qqt Queue, s string) {
	for _, ch := range []byte(s) {
		if !issafe(ch) {
			ch = '?'
		}
		qqt.WriteByte(ch)
	}
}

/* "Received: from relay1.uu.net (HELO uunet.uu.net) (7@192.48.96.5)\n" */
/* "  by silverton.berkeley.edu with SMTP; 26 Sep 1995 04:46:54 -0000\n" */

func (d *Server) received(qqt Queue, ss *session) error {
	qqt.WriteString("Received: from ")
	safeput(qqt, ss.remoteHost)
	if ss.helohost != "" {
		qqt.WriteString(" (HELO ")
		safeput(qqt, ss.helohost)
		qqt.WriteString(")")
	}
	qqt.WriteString(" (")
	if ss.user != "" {
		safeput(qqt, ss.user)
		qqt.WriteString("@")
	}
	safeput(qqt, ss.remoteIP)
	qqt.WriteString(")\n  by ")
	safeput(qqt, ss.local)
	qqt.WriteString(" with ")
	qqt.WriteString(ss.proto)
	qqt.WriteString("; ")
	dt := time.Now()
	qqt.WriteString(dt.Format("2 Jan 2006 15:04:05 -0700"))
	return qqt.WriteByte('\n')
}
