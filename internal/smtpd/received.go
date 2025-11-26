package smtpd

import "time"

func isSafe(ch byte) bool {
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

func safePut(qqt Queue, s string) {
	for _, ch := range []byte(s) {
		if !isSafe(ch) {
			ch = '?'
		}
		qqt.WriteByte(ch)
	}
}

func received(qqt Queue, ss *session) error {
	// Received: from relay1.uu.net (HELO uunet.uu.net) (7@192.48.96.5)
	//   by silverton.berkeley.edu with SMTP; 26 Sep 1995 04:46:54 -0000

	qqt.WriteString("Received: from ")
	safePut(qqt, ss.remoteHosts)

	if ss.state.heloHost != "" {
		qqt.WriteString(" (HELO ")
		safePut(qqt, ss.state.heloHost)
		qqt.WriteString(")")
	}

	qqt.WriteString(" (")
	if ss.state.username != "" {
		safePut(qqt, ss.state.username)
		qqt.WriteString("@")
	}
	safePut(qqt, ss.remoteIP)
	qqt.WriteString(")\n  by ")

	safePut(qqt, ss.local)
	qqt.WriteString(" with ")
	qqt.WriteString(ss.proto)
	qqt.WriteString("; ")

	dt := time.Now()
	qqt.WriteString(dt.Format("2 Jan 2006 15:04:05 -0700"))

	return qqt.WriteByte('\n')
}
