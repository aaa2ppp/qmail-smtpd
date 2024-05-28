package smtpd

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"qmail-smtpd/internal/commands"
	"qmail-smtpd/internal/safeio"
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

type Smtpd struct {
	Greeting      string
	Databytes     int
	Timeout       time.Duration
	RemoteIP      string
	RemoteHost    string
	RemoteInfo    string
	LocalIPHost   string
	LocalHost     string
	RelayClient   string
	RelayClientOk bool
	RcptHosts     AddrMatcher
	BadMailFrom   AddrMatcher
	IPMe          IPMe
	Qmail         Qmail
	Hostname      string
	Auth          Authenticator

	ssin  *bufio.Reader
	ssout *bufio.Writer

	helohost        string
	fakehelo        string /* pointer into helohost, or 0 */
	seenmail        bool
	flagbarf        bool /* defined if seenmail */
	mailfrom        string
	rcptto          []string
	bytestooverflow uint
	qqt             QmailQueue
	authorized      bool
}

func (d *Smtpd) flush() {
	if err := d.ssout.Flush(); err != nil {
		_exit(1)
	}
}

func (d *Smtpd) out(s string) {
	if _, err := d.ssout.WriteString(s); err != nil {
		_exit(1)
	}
}

func (d *Smtpd) die_read()  { _exit(1) }
func (d *Smtpd) die_alarm() { d.out("451 timeout (#4.4.2)\r\n"); d.flush(); _exit(1) }
func (d *Smtpd) die_nomem() { d.out("421 out of memory (#4.3.0)\r\n"); d.flush(); _exit(1) }

func (d *Smtpd) straynewline() {
	d.out("451 See http://pobox.com/~djb/docs/smtplf.html.\r\n")
	d.flush()
	_exit(1)
}

func (d *Smtpd) err_bmf() {
	d.out("553 sorry, your envelope sender is in my badmailfrom list (#5.7.1)\r\n")
}
func (d *Smtpd) err_nogateway() {
	d.out("553 sorry, that domain isn't in my list of allowed rcpthosts (#5.7.1)\r\n")
}
func (d *Smtpd) err_unimpl()   { d.out("502 unimplemented (#5.5.1)\r\n") }
func (d *Smtpd) err_syntax()   { d.out("555 syntax error (#5.5.4)\r\n") }
func (d *Smtpd) err_wantmail() { d.out("503 MAIL first (#5.5.1)\r\n") }
func (d *Smtpd) err_wantrcpt() { d.out("503 RCPT first (#5.5.1)\r\n") }
func (d *Smtpd) err_noop()     { d.out("250 ok\r\n") }
func (d *Smtpd) err_vrfy()     { d.out("252 send some mail, i'll try my best\r\n") }
func (d *Smtpd) err_qqt()      { d.out("451 qqt failure (#4.3.0)\r\n") }

func (d *Smtpd) smtp_greet(code string) {
	d.out(code)
	d.out(d.Greeting)
}

func (d *Smtpd) smtp_help(_ string) {
	d.out("214 qmail home page: http://pobox.com/~djb/qmail.html\r\n")
}

func (d *Smtpd) smtp_quit(_ string) {
	d.smtp_greet("221 ")
	d.out("\r\n")
	d.flush()
	_exit(0)
}

func (d *Smtpd) dohelo(arg string) {
	d.helohost = arg
	if !strings.EqualFold(d.RemoteHost, d.helohost) {
		d.fakehelo = d.helohost
	}
}

func (d *Smtpd) smtp_helo(arg string) {
	d.smtp_greet("250 ")
	d.out("\r\n")
	d.seenmail = false
	d.dohelo(arg)
}

func (d *Smtpd) smtp_ehlo(arg string) {
	d.smtp_greet("250-")
	if d.Auth != nil {
		d.out("\r\n250-AUTH LOGIN CRAM-MD5 PLAIN")
		d.out("\r\n250-AUTH=LOGIN CRAM-MD5 PLAIN")
	}
	d.out("\r\n250-PIPELINING\r\n250 8BITMIME\r\n")
	d.seenmail = false
	d.dohelo(arg)
}

func (d *Smtpd) smtp_rset(args string) {
	d.seenmail = false
	d.out("250 flushed\r\n")
}

func (d *Smtpd) smtp_mail(arg string) {
	addr, ok := addrparse(arg)
	if !ok {
		d.err_syntax()
		return
	}
	if d.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.LocalIPHost, d.IPMe)
	}
	d.flagbarf = d.BadMailFrom != nil && d.BadMailFrom.Match(addr)
	d.seenmail = true
	d.rcptto = d.rcptto[:0]
	d.mailfrom = addr
	d.out("250 ok\r\n")
}

func (d *Smtpd) smtp_rcpt(arg string) {
	if !d.seenmail {
		d.err_wantmail()
		return
	}
	addr, ok := addrparse(arg)
	if !ok {
		d.err_syntax()
		return
	}
	if d.LocalIPHost != "" {
		addr = replaceLocalIP(addr, d.LocalIPHost, d.IPMe)
	}
	if d.flagbarf {
		d.err_bmf()
		return
	}
	if d.RelayClientOk {
		addr += d.RelayClient
	} else {
		if d.RcptHosts != nil && !d.RcptHosts.Match(addr) {
			d.err_nogateway()
			return
		}
	}
	d.rcptto = append(d.rcptto, addr)
	d.out("250 ok\r\n")
}

func (d *Smtpd) acceptmessage(qp int) {
	when := time.Now()
	d.out("250 ok ")
	d.out(strconv.Itoa(int(when.Unix())))
	d.out(" qt ")
	d.out(strconv.Itoa(qp))
	d.out("\r\n")
}

func (d *Smtpd) smtp_data(_ string) {
	if !d.seenmail {
		d.err_wantmail()
		return
	}
	if len(d.rcptto) == 0 {
		d.err_wantrcpt()
		return
	}
	d.seenmail = false
	if d.Qmail == nil {
		d.err_qqt()
		return
	}
	var err error
	d.qqt, err = d.Qmail.Open()
	if err != nil {
		d.err_qqt()
		return
	}
	qp := d.qqt.Pid()
	d.out("354 go ahead\r\n")

	received(d.qqt, "SMTP", d.LocalHost, d.RemoteIP, d.RemoteHost, d.RemoteInfo, d.fakehelo)

	if d.Databytes != 0 {
		d.bytestooverflow = uint(d.Databytes) + 1
	}
	hops := d.blast()

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
		d.acceptmessage(qp)
		return
	}
	if too_many_hops {
		d.out("554 too many hops, this message is looping (#5.4.6)\r\n")
		return
	}
	if d.Databytes != 0 && d.bytestooverflow == 0 {
		d.out("552 sorry, that message size exceeds my databytes limit (#5.3.4)\r\n")
		return
	}
	if qqx[0] == 'D' {
		d.out("554 ")
	} else {
		d.out("451 ")
	}
	d.out(qqx[1:])
	d.out("\r\n")
}

func cmd_fun(fn func()) func(string) {
	return func(_ string) { fn() }
}

func (d *Smtpd) Run(r io.Reader, w io.Writer) (err error) {

	// XXX catch _exit
	defer func() {
		if p := recover(); p != nil {
			if code, ok := p.(exitCode); ok {
				if int(code) != 0 {
					err = errors.New("exit with code " + strconv.Itoa(int(code)))
				}
				return
			}
			panic(p)
		}
	}()

	timeout := d.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	d.ssin = bufio.NewReader(safeio.NewReader(r, timeout, d.flush))
	d.ssout = bufio.NewWriter(safeio.NewWriter(w, timeout))

	d.dohelo(d.RemoteHost)
	d.smtp_greet("200 ")
	d.out(" ESMTP\r\n")

	cmds := []commands.Command{
		{"rcpt", d.smtp_rcpt, nil},
		{"mail", d.smtp_mail, nil},
		{"data", d.smtp_data, d.flush},
		{"auth", d.smtp_auth, d.flush},
		{"quit", d.smtp_quit, d.flush},
		{"helo", d.smtp_helo, d.flush},
		{"ehlo", d.smtp_ehlo, d.flush},
		{"rset", d.smtp_rset, nil},
		{"help", d.smtp_help, d.flush},
		{"noop", cmd_fun(d.err_noop), d.flush}, // WTF? почему err_noop, а не smtp_noop?
		{"vrfy", cmd_fun(d.err_vrfy), d.flush}, // WTF? аналогично?
		{"", cmd_fun(d.err_unimpl), d.flush},
	}

	if err := commands.Loop(d.ssin, cmds); err != nil {
		if err == safeio.ErrIOTimeout {
			d.die_alarm()
		}
		d.die_read()
	}
	d.die_nomem()

	return nil // stub
}
