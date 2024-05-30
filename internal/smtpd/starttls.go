package smtpd

import (
	"crypto/tls"
)

func (d *Smtpd) smtp_tls(arg string) {
	if d.TLSConfig == nil || d.tlsEnabled {
		d.err_unimpl()
		return
	}
	if arg != "" {
		d.out("501 Syntax error (no parameters allowed) (#5.5.4)\r\n")
		return
	}
	d.out("220 go ahead\r\n")
	d.flush()
	conn := tls.Server(d.conn, d.TLSConfig)
	d.safeio_init(conn)
	d.dohelo(d.RemoteHost)
	d.tlsEnabled = true
}
