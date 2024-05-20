package main

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

func safeput(qqt *tQmail, s string) {
	for _, ch := range []byte(s) {
		if !issafe(ch) {
			ch = '?'
		}
		qmail_putc(qqt, ch)
	}
}

/* "Received: from relay1.uu.net (HELO uunet.uu.net) (7@192.48.96.5)\n" */
/* "  by silverton.berkeley.edu with SMTP; 26 Sep 1995 04:46:54 -0000\n" */

func received(
	qqt *tQmail,
	protocol string,
	local string,
	remoteip string,
	remotehost string,
	remoteinfo string,
	helo string,
) {
	qmail_puts(qqt, "Received: from ")
	safeput(qqt, remotehost)
	if helo != "" {
		qmail_puts(qqt, " (HELO ")
		safeput(qqt, helo)
		qmail_puts(qqt, ")")
	}
	qmail_puts(qqt, " (")
	if remoteinfo != "" {
		safeput(qqt, remoteinfo)
		qmail_puts(qqt, "@")
	}
	safeput(qqt, remoteip)
	qmail_puts(qqt, ")\n  by ")
	safeput(qqt, local)
	qmail_puts(qqt, " with ")
	qmail_puts(qqt, protocol)
	qmail_puts(qqt, "; ")
	dt := time.Now()
	qmail_puts(qqt, dt.Format(time.RFC822Z))
	qmail_putc(qqt, '\n')
}
