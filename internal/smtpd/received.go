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

func received(
	qqt Queue,
	protocol string,
	local string,
	remoteip string,
	remotehost string,
	remoteinfo string,
	helo string,
) error {
	qqt.WriteString("Received: from ")
	safeput(qqt, remotehost)
	if helo != "" {
		qqt.WriteString(" (HELO ")
		safeput(qqt, helo)
		qqt.WriteString(")")
	}
	qqt.WriteString(" (")
	if remoteinfo != "" {
		safeput(qqt, remoteinfo)
		qqt.WriteString("@")
	}
	safeput(qqt, remoteip)
	qqt.WriteString(")\n  by ")
	safeput(qqt, local)
	qqt.WriteString(" with ")
	qqt.WriteString(protocol)
	qqt.WriteString("; ")
	dt := time.Now()
	qqt.WriteString(dt.Format("2 Jan 2006 15:04:05 -0700"))
	return qqt.WriteByte('\n')
}
