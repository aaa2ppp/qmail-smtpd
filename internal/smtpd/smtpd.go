package smtpd

import (
	"cmp"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"qmail-smtpd/internal/smtpd/interfaces"
	"qmail-smtpd/internal/smtpd/safeio"
)

type (
	AddrMatcher = interfaces.AddrMatcher
	IPMe        = interfaces.IPMe
	Qmail       = interfaces.Qmail
	QmailQueue  = interfaces.QmailQueue
	LogWriter   = interfaces.LogWriter
)

const (
	MaxHops        = 100
	DefaultTimeout = 1200 * time.Second // WTF: why so many?
)

var ErrClientQuit = errors.New("client quit")

type Config struct {
	Greeting      string
	Databytes     int
	Timeout       time.Duration
	RemoteIP      string
	RemoteHost    string
	LocalIPHost   string
	LocalIP       string
	LocalHost     string
	RelayClient   string
	RelayClientOk bool
	RcptHosts     AddrMatcher
	BadMailFrom   AddrMatcher
	MbxHosts      AddrMatcher
	IPMe          IPMe
	Qmail         Qmail
	Hostname      string
	Auth          Authenticator
	TLSConfig     *tls.Config
	Logger        LogWriter
}

type sessionState struct {
	// remoteInfo contains the authenticated username in the same way as in qmail-smtpd with auth patch.
	// It set only after successful AUTH and is never set externally. Before calling `qmail-queue`,
	// the TCPREMOTEINFO environment variable will be set from it.
	remoteInfo string

	relayClient   string
	relayClientOk bool

	helohost        string
	fakehelo        string /* pointer into helohost, or 0 */
	seenmail        bool
	flagbarf        bool /* defined if seenmail */
	mailfrom        string
	rcptto          []string
	qqt             QmailQueue
	authorized      bool
	tlsEnabled      bool
}

type session struct {
	*safeio.SafeIO
	sessionState
}

func (ss *session) out(s string) error {
	_, err := ss.WriteString(s)
	return err
}

type Server struct {
	cfg      *Config
	cmdTable map[string]command
}

func NewServer(cfg *Config) *Server {
	d := &Server{cfg: cfg}
	d.cmdTable = newCommandTable(d)
	return d
}

func (d *Server) err_bmf(ss *session) error {
	return ss.out("553 sorry, your envelope sender is in my badmailfrom list (#5.7.1)\r\n")
}
func (d *Server) err_nogateway(ss *session) error {
	return ss.out("553 sorry, that domain isn't in my list of allowed rcpthosts (#5.7.1)\r\n")
}
func (d *Server) err_unimpl(ss *session, _ string) error {
	return ss.out("502 unimplemented (#5.5.1)\r\n")
}
func (d *Server) err_syntax(ss *session) error         { return ss.out("555 syntax error (#5.5.4)\r\n") }
func (d *Server) err_wantmail(ss *session) error       { return ss.out("503 MAIL first (#5.5.1)\r\n") }
func (d *Server) err_wantrcpt(ss *session) error       { return ss.out("503 RCPT first (#5.5.1)\r\n") }
func (d *Server) err_noop(ss *session, _ string) error { return ss.out("250 ok\r\n") }
func (d *Server) err_vrfy(ss *session, _ string) error {
	return ss.out("252 send some mail, i'll try my best\r\n")
}
func (d *Server) err_qqt(ss *session) error     { return ss.out("451 qqt failure (#4.3.0)\r\n") }
func (d *Server) err_timeout(ss *session) error { return ss.out("451 timeout (#4.4.2)\r\n") }

func (d *Server) smtp_greet(ss *session, code string) error {
	_ = ss.out(code)
	return ss.out(d.cfg.Greeting)
}

func (d *Server) smtp_help(ss *session, _ string) error {
	return ss.out("214 qmail home page: http://pobox.com/~djb/qmail.html\r\n")
}

func (d *Server) smtp_quit(ss *session, _ string) error {
	_ = d.smtp_greet(ss, "221 ")
	_ = ss.out("\r\n")
	return cmp.Or(ss.Flush(), ErrClientQuit)
}

func (d *Server) dohelo(ss *session, arg string) {
	ss.helohost = arg
	if !strings.EqualFold(d.cfg.RemoteHost, ss.helohost) {
		ss.fakehelo = ss.helohost
	}
}

func (d *Server) smtp_helo(ss *session, arg string) error {
	ss.seenmail = false
	d.dohelo(ss, arg)
	_ = d.smtp_greet(ss, "250 ")
	return ss.out("\r\n")
}

func (d *Server) smtp_ehlo(ss *session, arg string) error {
	ss.seenmail = false
	d.dohelo(ss, arg)
	_ = d.smtp_greet(ss, "250-")
	if d.cfg.Auth != nil && !ss.authorized {
		if ss.tlsEnabled {
			_ = ss.out("\r\n250-AUTH LOGIN CRAM-MD5 PLAIN")
			_ = ss.out("\r\n250-AUTH=LOGIN CRAM-MD5 PLAIN") // WTF? =
		} else {
			_ = ss.out("\r\n250-AUTH CRAM-MD5")
			_ = ss.out("\r\n250-AUTH=CRAM-MD5") // WTF? =
		}
	}
	if d.cfg.Databytes > 0 {
		_ = ss.out("\r\n250-SIZE ")
		_ = ss.out(strconv.Itoa(d.cfg.Databytes))
	}
	if d.cfg.TLSConfig != nil && !ss.tlsEnabled {
		_ = ss.out("\r\n250-STARTTLS")
	}
	_ = ss.out("\r\n250-PIPELINING")
	return ss.out("\r\n250 8BITMIME\r\n")
}

func (d *Server) smtp_rset(ss *session, args string) error {
	ss.seenmail = false
	return ss.out("250 flushed\r\n")
}

func (d *Server) smtp_mail(ss *session, arg string) error {
	addr, ok := addrparse(arg)
	if !ok {
		return d.err_syntax(ss)
	}
	if d.cfg.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.cfg.LocalIPHost, d.cfg.IPMe)
	}
	// TODO: check SIZE parameter
	ss.flagbarf = d.cfg.BadMailFrom != nil && d.cfg.BadMailFrom.Match(addr)
	ss.seenmail = true
	ss.rcptto = ss.rcptto[:0]
	ss.mailfrom = addr
	return ss.out("250 ok\r\n")
}

func (d *Server) smtp_rcpt(ss *session, arg string) error {
	if !ss.seenmail {
		return d.err_wantmail(ss)
	}
	addr, ok := addrparse(arg)
	if !ok {
		return d.err_syntax(ss)
	}
	if d.cfg.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.cfg.LocalIPHost, d.cfg.IPMe)
	}
	if ss.flagbarf {
		return d.err_bmf(ss)
	}
	if ss.relayClientOk {
		addr += ss.relayClient
	} else {
		if d.cfg.RcptHosts != nil && !d.cfg.RcptHosts.Match(addr) {
			return d.err_nogateway(ss)
		}
		// Дополнительная проверка: если домен в mbxhosts, проверить существование ящика
		if d.cfg.MbxHosts != nil && !d.cfg.MbxHosts.Match(addr) {
			return ss.out("553 mailbox does not exist (#5.1.1)\r\n")
		}
	}
	ss.rcptto = append(ss.rcptto, addr)
	return ss.out("250 ok\r\n")
}

func (d *Server) acceptmessage(ss *session, qp int) error {
	when := time.Now()
	_ = ss.out("250 ok ")
	_ = ss.out(strconv.Itoa(int(when.Unix())))
	_ = ss.out(" qt ")
	_ = ss.out(strconv.Itoa(qp))
	return ss.out("\r\n")
}

func (d *Server) prepareQmailEnv(ss *session) []string {
	env := []string{
		"TCPREMOTEIP=" + d.cfg.RemoteIP,
		"TCPREMOTEHOST=" + d.cfg.RemoteHost,
		"PROTO=SMTP",
		"DATABYTES=" + strconv.Itoa(d.cfg.Databytes),
	}
	if ss.authorized {
		env = append(env, "TCPREMOTEINFO="+ss.remoteInfo)
	}
	if ss.relayClientOk {
		env = append(env, "RELAYCLIENT="+ss.relayClient)
	}
	if v, ok := os.LookupEnv("QMAILQUEUE"); ok {
		env = append(env, "QMAILQUEUE="+v)
	}
	return env
}

func (d *Server) smtp_data(ss *session, _ string) error {
	if !ss.seenmail {
		return d.err_wantmail(ss)
	}
	if len(ss.rcptto) == 0 {
		return d.err_wantrcpt(ss)
	}
	ss.seenmail = false
	if d.cfg.Qmail == nil {
		return d.err_qqt(ss)
	}
	var err error
	ss.qqt, err = d.cfg.Qmail.Open(d.prepareQmailEnv(ss))
	if err != nil {
		return d.err_qqt(ss)
	}
	qp := ss.qqt.Pid()
	ss.out("354 go ahead\r\n")

	received(ss.qqt, "SMTP", d.cfg.LocalHost, d.cfg.RemoteIP, d.cfg.RemoteHost, ss.remoteInfo, ss.fakehelo)

	hops, overflow, err := d.blast(ss, d.cfg.Databytes)
	if err != nil {
		return err
	}
	// TODO: log received data bytes

	too_many_hops := hops >= MaxHops
	if too_many_hops {
		ss.qqt.Fail()
	}

	ss.qqt.From(ss.mailfrom)
	for _, it := range ss.rcptto {
		ss.qqt.To(it)
	}

	qqx := ss.qqt.Close()
	ss.qqt = nil

	if qqx == "" {
		return d.acceptmessage(ss, qp)
	}
	if too_many_hops {
		return ss.out("554 too many hops, this message is looping (#5.4.6)\r\n")
	}
	if d.cfg.Databytes > 0 && overflow == 0 {
		return ss.out("552 sorry, that message size exceeds my databytes limit (#5.3.4)\r\n")
	}

	if qqx[0] == 'D' {
		_ = ss.out("554 ")
	} else {
		_ = ss.out("451 ")
	}
	_ = ss.out(qqx[1:])
	return ss.out("\r\n")
}

type handlerFunc = func(ss *session, arg string) error

type command struct {
	handler   handlerFunc
	needFlush bool
}

type commandTable map[string]command

const unimpl = "*unimpl*"

func newCommandTable(d *Server) commandTable {
	return map[string]command{
		"rcpt":     {d.smtp_rcpt, false},
		"mail":     {d.smtp_mail, false},
		"data":     {d.smtp_data, true},
		"auth":     {d.smtp_auth, true},
		"quit":     {d.smtp_quit, true},
		"helo":     {d.smtp_helo, true},
		"ehlo":     {d.smtp_ehlo, true},
		"rset":     {d.smtp_rset, false},
		"help":     {d.smtp_help, true},
		"starttls": {d.smtp_tls, false},
		"noop":     {d.err_noop, true},   // WTF: почему err_noop, а не smtp_noop?
		"vrfy":     {d.err_vrfy, true},   // WTF: аналогично?
		unimpl:     {d.err_unimpl, true}, // WTF: аналогично?
	}
}

func (d *Server) newSession(conn net.Conn) *session {
	return &session{
		SafeIO: safeio.New(conn, d.cfg.Logger, d.cfg.Timeout),
		sessionState: sessionState{
			relayClient:   d.cfg.RelayClient,
			relayClientOk: d.cfg.RelayClientOk,
		},
	}
}

func (d *Server) run(ss *session) error {
	d.dohelo(ss, d.cfg.RemoteHost)
	d.smtp_greet(ss, "220 ")
	ss.out(" ESMTP\r\n")

	err := d.commandLoop(ss) // always return error

	if err == ErrClientQuit {
		return nil
	}

	if errors.Is(err, os.ErrDeadlineExceeded) {
		d.err_timeout(ss)
		ss.Flush()
	}

	return err
}

func (d *Server) Run(conn net.Conn) error {
	ss := d.newSession(conn)
	return d.run(ss)
}
