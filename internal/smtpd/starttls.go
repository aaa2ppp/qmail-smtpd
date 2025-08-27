package smtpd

import (
	"crypto/tls"
)

func smtp_tls(d *Smtpd,arg string) error {
	if d.TLSConfig == nil || d.tlsEnabled {
		return smtp_unimpl(d, "") // или 454 TLS not available
	}

	// if d.tlsEnabled {
	// 	return d.out("503 Duplicate STARTTLS (#5.5.1)\r\n")
	// }

	if arg != "" {
		return d.out("501 Syntax error (no parameters allowed) (#5.5.4)\r\n")
	}

	_ = d.out("220 Ready to start TLS\r\n")
	if err := d.flush(); err != nil {
		return err
	}

	conn := tls.Server(d.conn, d.TLSConfig)
	d.initIO(conn)
	d.dohelo(d.RemoteHost)
	d.tlsEnabled = true

	return nil
}
