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

func safeput(qqt QmailQueue, s string) {
	for _, ch := range []byte(s) {
		if !issafe(ch) {
			ch = '?'
		}
		qqt.Putc(ch)
	}
}

/* "Received: from relay1.uu.net (HELO uunet.uu.net) (7@192.48.96.5)\n" */
/* "  by silverton.berkeley.edu with SMTP; 26 Sep 1995 04:46:54 -0000\n" */

func received(
	qqt QmailQueue,
	protocol string,
	local string,
	remoteip string,
	remotehost string,
	remoteinfo string,
	helo string,
) {
	qqt.Puts("Received: from ")
	safeput(qqt, remotehost)
	if helo != "" {
		qqt.Puts(" (HELO ")
		safeput(qqt, helo)
		qqt.Puts(")")
	}
	qqt.Puts(" (")
	if remoteinfo != "" {
		safeput(qqt, remoteinfo)
		qqt.Puts("@")
	}
	safeput(qqt, remoteip)
	qqt.Puts(")\n  by ")
	safeput(qqt, local)
	qqt.Puts(" with ")
	qqt.Puts(protocol)
	qqt.Puts("; ")
	dt := time.Now()
	qqt.Puts(dt.Format(time.RFC822Z))
	qqt.Putc('\n')
}
