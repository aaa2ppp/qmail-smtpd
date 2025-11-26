package smtpd

func smtp_starttls(ss *session, arg string) error {
	if ss.tlsConfig == nil {
		return smtp_unimpl(ss, "") // или 454 TLS not available
	}

	if ss.tlsEnabled {
		return ss.out("503 Duplicate STARTTLS (#5.5.1)\r\n")
	}

	if ss.state.seenMail {
		return ss.out("503 STARTTLS invalid during mail transaction (#5.5.1)\r\n")
	}

	if arg != "" {
		return ss.out("501 Syntax error (no parameters allowed) (#5.5.4)\r\n")
	}

	_ = ss.out("220 Ready to start TLS\r\n")
	if err := ss.io.StartTLS(ss.tlsConfig); err != nil {
		return err
	}

	ss.tlsEnabled = true
	dohelo(ss, ss.remoteHosts)

	return nil
}
