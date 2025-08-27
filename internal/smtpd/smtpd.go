package smtpd

import (
	"bufio"
	"cmp"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"qmail-smtpd/internal/scan"
)

type AddrMatcher interface {
	Match(string) bool
}

type IPMe interface {
	Is(scan.IPAddress) bool
}

type Qmail interface {
	Open() (QmailQueue, error)
}

type LogWriter interface { // XXX
	io.StringWriter
	Flush() error
	WithPrefix(string) LogWriter
}

type QmailQueue interface {
	Pid() int
	Putc(byte)
	Puts(string)
	From(string)
	To(string)
	Fail()
	Close() string
}

const (
	MaxHops        = 100
	DefaultTimeout = 1200 * time.Second // WTF: why so many?
)

var ErrClientQuit = errors.New("client quit")

type Smtpd struct {
	Greeting      string
	Databytes     int
	Timeout       time.Duration
	RemoteIP      string
	RemoteHost    string
	RemoteInfo    string
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
	Log           LogWriter

	conn  net.Conn
	ssin  *bufio.Reader
	ssout *bufio.Writer

	login  LogWriter
	logout LogWriter

	helohost        string
	fakehelo        string /* pointer into helohost, or 0 */
	seenmail        bool
	flagbarf        bool /* defined if seenmail */
	mailfrom        string
	rcptto          []string
	bytestooverflow uint
	qqt             QmailQueue
	authorized      bool
	tlsEnabled      bool
}

func (d *Smtpd) flush() error {
	if d.logout != nil {
		d.logout.Flush()
	}
	return d.ssout.Flush()
}

func (d *Smtpd) out(s string) error {
	if d.logout != nil {
		d.logout.WriteString(s)
	}
	_, err := d.ssout.WriteString(s)
	return err
}

func (d *Smtpd) getln() (string, error) {
	s, err := d.ssin.ReadString('\n')
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			d.err_timeout()
			d.flush()
		}
		return "", err
	}
	if d.login != nil {
		d.login.WriteString(s)
		d.login.Flush()
	}
	s = s[:len(s)-1]
	if s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	return s, nil
}

func (d *Smtpd) err_bmf() error {
	return d.out("553 sorry, your envelope sender is in my badmailfrom list (#5.7.1)\r\n")
}
func (d *Smtpd) err_nogateway() error {
	return d.out("553 sorry, that domain isn't in my list of allowed rcpthosts (#5.7.1)\r\n")
}
func (d *Smtpd) err_unimpl(_ string) error { return d.out("502 unimplemented (#5.5.1)\r\n") }
func (d *Smtpd) err_syntax() error         { return d.out("555 syntax error (#5.5.4)\r\n") }
func (d *Smtpd) err_wantmail() error       { return d.out("503 MAIL first (#5.5.1)\r\n") }
func (d *Smtpd) err_wantrcpt() error       { return d.out("503 RCPT first (#5.5.1)\r\n") }
func (d *Smtpd) err_noop(_ string) error   { return d.out("250 ok\r\n") }
func (d *Smtpd) err_vrfy(_ string) error   { return d.out("252 send some mail, i'll try my best\r\n") }
func (d *Smtpd) err_qqt() error            { return d.out("451 qqt failure (#4.3.0)\r\n") }
func (d *Smtpd) err_timeout() error        { return d.out("451 timeout (#4.4.2)\r\n") }

func (d *Smtpd) smtp_greet(code string) error {
	_ = d.out(code)
	return d.out(d.Greeting)
}

func (d *Smtpd) smtp_help(_ string) error {
	return d.out("214 qmail home page: http://pobox.com/~djb/qmail.html\r\n")
}

func (d *Smtpd) smtp_quit(_ string) error {
	_ = d.smtp_greet("221 ")
	_ = d.out("\r\n")
	return cmp.Or(d.flush(), ErrClientQuit)
}

func (d *Smtpd) dohelo(arg string) {
	d.seenmail = false
	d.helohost = arg
	if !strings.EqualFold(d.RemoteHost, d.helohost) {
		d.fakehelo = d.helohost
	}
}

func (d *Smtpd) smtp_helo(arg string) error {
	d.dohelo(arg)
	_ = d.smtp_greet("250 ")
	return d.out("\r\n")
}

func (d *Smtpd) smtp_ehlo(arg string) error {
	d.dohelo(arg)
	_ = d.smtp_greet("250-")
	if d.Auth != nil && !d.authorized {
		if d.tlsEnabled {
			_ = d.out("\r\n250-AUTH LOGIN CRAM-MD5 PLAIN")
			_ = d.out("\r\n250-AUTH=LOGIN CRAM-MD5 PLAIN") // WTF? =
		} else {
			_ = d.out("\r\n250-AUTH CRAM-MD5")
			_ = d.out("\r\n250-AUTH=CRAM-MD5") // WTF? =
		}
	}
	if d.Databytes > 0 {
		_ = d.out("\r\n250-SIZE ")
		_ = d.out(strconv.Itoa(d.Databytes))
	}
	if d.TLSConfig != nil && !d.tlsEnabled {
		_ = d.out("\r\n250-STARTTLS")
	}
	_ = d.out("\r\n250-PIPELINING")
	return d.out("\r\n250 8BITMIME\r\n")
}

func (d *Smtpd) smtp_rset(args string) error {
	d.seenmail = false
	return d.out("250 flushed\r\n")
}

func (d *Smtpd) smtp_mail(arg string) error {
	addr, ok := addrparse(arg)
	if !ok {
		return d.err_syntax()
	}
	if d.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.LocalIPHost, d.IPMe)
	}
	// TODO: check SIZE parameter
	d.flagbarf = d.BadMailFrom != nil && d.BadMailFrom.Match(addr)
	d.seenmail = true
	d.rcptto = d.rcptto[:0]
	d.mailfrom = addr
	return d.out("250 ok\r\n")
}

func (d *Smtpd) smtp_rcpt(arg string) error {
	if !d.seenmail {
		return d.err_wantmail()
	}
	addr, ok := addrparse(arg)
	if !ok {
		return d.err_syntax()
	}
	if d.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.LocalIPHost, d.IPMe)
	}
	if d.flagbarf {
		return d.err_bmf()
	}
	if d.RelayClientOk {
		addr += d.RelayClient
	} else {
		if d.RcptHosts != nil && !d.RcptHosts.Match(addr) {
			return d.err_nogateway()
		}
		// Дополнительная проверка: если домен в mbxhosts, проверить существование ящика
		if d.MbxHosts != nil && !d.MbxHosts.Match(addr) {
			return d.out("553 mailbox does not exist (#5.1.1)\r\n")
		}
	}
	d.rcptto = append(d.rcptto, addr)
	return d.out("250 ok\r\n")
}

func (d *Smtpd) acceptmessage(qp int) error {
	when := time.Now()
	_ = d.out("250 ok ")
	_ = d.out(strconv.Itoa(int(when.Unix())))
	_ = d.out(" qt ")
	_ = d.out(strconv.Itoa(qp))
	return d.out("\r\n")
}

func (d *Smtpd) smtp_data(_ string) error {
	if !d.seenmail {
		return d.err_wantmail()
	}
	if len(d.rcptto) == 0 {
		return d.err_wantrcpt()
	}
	d.seenmail = false
	if d.Qmail == nil {
		return d.err_qqt()
	}
	var err error
	d.qqt, err = d.Qmail.Open()
	if err != nil {
		return d.err_qqt()
	}
	qp := d.qqt.Pid()
	d.out("354 go ahead\r\n")

	received(d.qqt, "SMTP", d.LocalHost, d.RemoteIP, d.RemoteHost, d.RemoteInfo, d.fakehelo)

	if d.Databytes != 0 {
		d.bytestooverflow = uint(d.Databytes) + 1
	}
	hops, err := d.blast()
	if err != nil {
		return err
	}
	// TODO: log received data bytes

	too_many_hops := hops >= MaxHops
	if too_many_hops {
		d.qqt.Fail()
	}

	d.qqt.From(d.mailfrom)
	for _, it := range d.rcptto {
		d.qqt.To(it)
	}

	qqx := d.qqt.Close()
	d.qqt = nil

	if qqx == "" {
		return d.acceptmessage(qp)
	}
	if too_many_hops {
		return d.out("554 too many hops, this message is looping (#5.4.6)\r\n")
	}
	if d.Databytes != 0 && d.bytestooverflow == 0 {
		return d.out("552 sorry, that message size exceeds my databytes limit (#5.3.4)\r\n")
	}

	if qqx[0] == 'D' {
		_ = d.out("554 ")
	} else {
		_ = d.out("451 ")
	}
	_ = d.out(qqx[1:])
	return d.out("\r\n")
}

type handlerFunc = func(arg string) error

type command struct {
	handler   handlerFunc
	needFlush bool
}

const unimpl = "unimpl"

func (d *Smtpd) createCommadsTable() map[string]command {
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

func (d *Smtpd) Run(conn net.Conn) error {
	if d.Log != nil {
		d.login = d.Log.WithPrefix("=> ")
		d.logout = d.Log.WithPrefix("<= ")
	}

	d.initIO(conn)

	d.dohelo(d.RemoteHost)
	d.smtp_greet("220 ")
	d.out(" ESMTP\r\n")

	ct := d.createCommadsTable()
	if err := d.commands(ct); err != nil && err != ErrClientQuit {
		return err
	}

	return nil
}
