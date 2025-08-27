package smtpd

import (
	"crypto/tls"
)

func (d *Smtpd) smtp_tls(arg string) error {
	if d.cfg.TLSConfig == nil {
		return d.err_unimpl("") // или 454 TLS not available
	}

	if d.tlsEnabled {
		return d.out("503 Duplicate STARTTLS (#5.5.1)\r\n")
	}

	if d.seenmail {
		return d.out("503 STARTTLS invalid during mail transaction (#5.5.1)\r\n")
	}

	if arg != "" {
		return d.out("501 Syntax error (no parameters allowed) (#5.5.4)\r\n")
	}

	_ = d.out("220 Ready to start TLS\r\n")
	if err := d.flush(); err != nil {
		return err
	}

	conn := tls.Server(d.conn, d.cfg.TLSConfig)
	d.initIO(conn)

	d.tlsEnabled = true
	d.seenmail = false
	d.resetAuthorized()

	/* have to discard the pre-STARTTLS HELO/EHLO argument, if any */
	d.dohelo(d.cfg.RemoteHost)

	return nil
}
