package smtpd

func (d *Server) err_bmf(ss *session) error {
	return ss.out("553 sorry, your envelope sender is in my badmailfrom list (#5.7.1)\r\n")
}
func (d *Server) err_nogateway(ss *session) error {
	return ss.out("553 sorry, that domain isn't in my list of allowed rcpthosts (#5.7.1)\r\n")
}
func (d *Server) err_unimpl(ss *session, _ string) error {
	return ss.out("502 unimplemented (#5.5.1)\r\n")
}
func (d *Server) err_syntax(ss *session) error   { return ss.out("555 syntax error (#5.5.4)\r\n") }
func (d *Server) err_wantmail(ss *session) error { return ss.out("503 MAIL first (#5.5.1)\r\n") }
func (d *Server) err_wantrcpt(ss *session) error { return ss.out("503 RCPT first (#5.5.1)\r\n") }

// WTF: djb, почему это ошибки? обрати внимание, что сигнатура соответствует хэндлеру, а коды 2xx
func (d *Server) err_noop(ss *session, _ string) error { return ss.out("250 ok\r\n") }
func (d *Server) err_vrfy(ss *session, _ string) error {
	return ss.out("252 send some mail, i'll try my best\r\n")
}

func (d *Server) err_qqt(ss *session) error     { return ss.out("451 qqt failure (#4.3.0)\r\n") }
func (d *Server) err_timeout(ss *session) error { return ss.out("451 timeout (#4.4.2)\r\n") }
