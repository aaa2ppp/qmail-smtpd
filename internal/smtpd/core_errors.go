package smtpd

func err_syntax(ss *session) error   { return ss.out("555 syntax error (#5.5.4)\r\n") }
func err_wantmail(ss *session) error { return ss.out("503 MAIL first (#5.5.1)\r\n") }
func err_wantrcpt(ss *session) error { return ss.out("503 RCPT first (#5.5.1)\r\n") }
func err_qqt(ss *session) error      { return ss.out("451 qqt failure (#4.3.0)\r\n") }
func err_timeout(ss *session) error  { return ss.out("451 timeout (#4.4.2)\r\n") }
