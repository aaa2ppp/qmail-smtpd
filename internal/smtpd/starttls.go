package smtpd

func (d *Server) smtp_tls(ss *session, arg string) error {
	if d.cfg.TLSConfig == nil {
		return d.err_unimpl(ss, "") // или 454 TLS not available
	}

	if ss.tlsEnabled {
		return ss.out("503 Duplicate STARTTLS (#5.5.1)\r\n")
	}

	if ss.seenmail {
		return ss.out("503 STARTTLS invalid during mail transaction (#5.5.1)\r\n")
	}

	if arg != "" {
		return ss.out("501 Syntax error (no parameters allowed) (#5.5.4)\r\n")
	}

	_ = ss.out("220 Ready to start TLS\r\n")
	if err := ss.StartTLS(d.cfg.TLSConfig); err != nil {
		return err
	}

	ss.tlsEnabled = true
	ss.seenmail = false
	d.resetAuthorized(ss)

	// have to discard the pre-STARTTLS HELO/EHLO argument, if any
	d.dohelo(ss, ss.remoteHost)

	return nil
}
