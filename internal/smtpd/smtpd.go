package smtpd

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"qmail-smtpd/internal/commands"
	"qmail-smtpd/internal/control"
	"qmail-smtpd/internal/control/badmailfrom"
	"qmail-smtpd/internal/control/ipme"
	"qmail-smtpd/internal/control/rcpthosts"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/safeio"
	"qmail-smtpd/internal/scan"
)

const (
	maxHops        = 100
	defaultTimeout = 1200 * time.Second // WTF: why so many?
)

type Smtpd struct {
	greeting      string
	databytes     int
	timeout       time.Duration
	remoteip      string
	remotehost    string
	remoteinfo    string
	local         string
	relayclient   string
	relayclientok bool
	helohost      string
	fakehelo      string /* pointer into helohost, or 0 */
	liphostok     bool
	liphost       string

	ssin  *bufio.Reader
	ssout *bufio.Writer

	seenmail        bool
	flagbarf        bool /* defined if seenmail */
	mailfrom        string
	rcptto          []string
	addr            string
	qqt             *qmail.Qmail
	bytestooverflow uint
}

func (sd *Smtpd) flush() {
	if err := sd.ssout.Flush(); err != nil {
		log.Fatal(err)
	}
}

func (sd *Smtpd) out(s string) {
	if _, err := sd.ssout.WriteString(s); err != nil {
		log.Fatal(err)
	}
}

func (sd *Smtpd) die_read()  { _exit(1) }
func (sd *Smtpd) die_alarm() { sd.out("451 timeout (#4.4.2)\r\n"); sd.flush(); _exit(1) }
func (sd *Smtpd) die_nomem() { sd.out("421 out of memory (#4.3.0)\r\n"); sd.flush(); _exit(1) }
func (sd *Smtpd) die_control() {
	sd.out("421 unable to read controls (#4.3.0)\r\n")
	sd.flush()
	_exit(1)
}
func (sd *Smtpd) die_ipme() {
	sd.out("421 unable to figure out my IP addresses (#4.3.0)\r\n")
	sd.flush()
	_exit(1)
}
func (sd *Smtpd) straynewline() {
	sd.out("451 See http://pobox.com/~djb/docs/smtplf.html.\r\n")
	sd.flush()
	_exit(1)
}

func (sd *Smtpd) err_bmf() {
	sd.out("553 sorry, your envelope sender is in my badmailfrom list (#5.7.1)\r\n")
}
func (sd *Smtpd) err_nogateway() {
	sd.out("553 sorry, that domain isn't in my list of allowed rcpthosts (#5.7.1)\r\n")
}
func (sd *Smtpd) err_unimpl()   { sd.out("502 unimplemented (#5.5.1)\r\n") }
func (sd *Smtpd) err_syntax()   { sd.out("555 syntax error (#5.5.4)\r\n") }
func (sd *Smtpd) err_wantmail() { sd.out("503 MAIL first (#5.5.1)\r\n") }
func (sd *Smtpd) err_wantrcpt() { sd.out("503 RCPT first (#5.5.1)\r\n") }
func (sd *Smtpd) err_noop()     { sd.out("250 ok\r\n") }
func (sd *Smtpd) err_vrfy()     { sd.out("252 send some mail, i'll try my best\r\n") }
func (sd *Smtpd) err_qqt()      { sd.out("451 qqt failure (#4.3.0)\r\n") }

func (sd *Smtpd) smtp_greet(code string) {
	sd.out(code)
	sd.out(sd.greeting)
}

func (sd *Smtpd) smtp_help(_ string) {
	sd.out("214 qmail home page: http://pobox.com/~djb/qmail.html\r\n")
}

func (sd *Smtpd) smtp_quit(_ string) {
	sd.smtp_greet("221 ")
	sd.out("\r\n")
	sd.flush()
	_exit(0)
}

func (sd *Smtpd) dohelo(arg string) {
	sd.helohost = arg
	if !strings.EqualFold(sd.remotehost, sd.helohost) {
		sd.fakehelo = sd.helohost
	}
}

func (sd *Smtpd) setup() {
	if control.Init() == -1 {
		sd.die_control()
	}

	if s, r := control.Rldef("control/smtpgreeting", true, ""); r != 1 {
		sd.die_control()
	} else {
		sd.greeting = s
	}

	if s, r := control.Rldef("control/localiphost", true, ""); r == -1 {
		sd.die_control()
	} else if r == 1 {
		sd.liphost = s
		sd.liphostok = true
	}

	sd.timeout = defaultTimeout
	if i, r := control.ReadInt("control/timeoutsmtpd"); r == -1 {
		sd.die_control()
	} else if r == 1 {
		if i <= 0 {
			i = 1
		}
		sd.timeout = time.Duration(i) * time.Second
	}

	if r := rcpthosts.Init(); r == -1 {
		sd.die_control()
	}

	if r := badmailfrom.Init(); r == -1 {
		sd.die_control()
	}

	if i, r := control.ReadInt("control/databytes"); r == -1 {
		sd.die_control()
	} else if r == 1 {
		sd.databytes = i
	}

	// x = env_get("DATABYTES");
	// if (x) { scan_ulong(x,&u); databytes = u; }
	// if (!(databytes + 1)) --databytes;  // WTF: if databytes == -1 then databytes = -2 ?
	if x := os.Getenv("DATABYTES"); x != "" {
		_, u := scan.ScanUlong(x)
		if u != 0 {
			sd.databytes = int(u)
		}
	}
	if sd.databytes+1 == 0 { // WTF?
		sd.databytes--
	}

	sd.remoteip = os.Getenv("TCPREMOTEIP")
	if sd.remoteip == "" {
		sd.remoteip = "unknown"
	}

	sd.local = os.Getenv("TCPLOCALHOST")
	if sd.local == "" {
		sd.local = os.Getenv("TCPLOCALIP")
	}
	if sd.local == "" {
		sd.local = "unknown"
	}

	sd.remotehost = os.Getenv("TCPREMOTEHOST")
	if sd.remotehost == "" {
		sd.remotehost = "unknown"
	}

	sd.remoteinfo = os.Getenv("TCPREMOTEINFO")
	sd.relayclient, sd.relayclientok = os.LookupEnv("RELAYCLIENT")

	sd.dohelo(sd.remotehost)
}

func (sd *Smtpd) addrparse(arg string) int {
	terminator := '>'

	if i := strings.IndexByte(arg, '<'); i != -1 {
		arg = arg[i+1:]
	} else { /* partner should go read rfc 821 */
		terminator = ' '
		if i := strings.IndexByte(arg, ':'); i != -1 {
			arg = arg[i+1:]
		}
		for len(arg) > 0 && arg[0] == ' ' {
			arg = arg[1:]
		}
	}

	/* strip source route */
	if len(arg) > 0 && arg[0] == '@' {
		for len(arg) > 0 && arg[0] != ':' {
			arg = arg[1:]
		}
	}

	var addr []byte
	var flagesc bool
	var flagquoted bool

	for _, ch := range []byte(arg) { /* copy arg to addr, stripping quotes */
		if flagesc {
			addr = append(addr, ch)
			flagesc = false
		} else {
			if !flagquoted && ch == byte(terminator) {
				break
			}
			switch ch {
			case '\\':
				flagesc = true
			case '"':
				flagquoted = !flagquoted
			default:
				addr = append(addr, ch)
			}
		}
	}
	/* could check for termination failure here, but why bother? */

	if sd.liphostok {
		i := bytes.LastIndexByte(addr, '@')
		if i != -1 { /* if not, partner should go read rfc 821 */
			if i+1 < len(addr) && addr[i+1] == '[' {
				l, ip := scan.ScanIPBracket(unsafeString(addr[i+1:]))
				if i+1+l == len(addr) {
					if ipme.Is(ip) {
						addr = append(addr[:i+1], sd.liphost...)
					}
				}
			}
		}
	}

	if len(addr) > 900 {
		return 0
	}

	sd.addr = string(addr)
	return 1
}

func (sd *Smtpd) addrallowed() bool {
	if !rcpthosts.Allowed(sd.addr) {
		sd.die_control()
	}
	return true
}

func (sd *Smtpd) smtp_helo(arg string) {
	sd.smtp_greet("250 ")
	sd.out("\r\n")
	sd.seenmail = false
	sd.dohelo(arg)
}

func (sd *Smtpd) smtp_ehlo(arg string) {
	sd.smtp_greet("250-")
	sd.out("\r\n250-PIPELINING\r\n250 8BITMIME\r\n")
	sd.seenmail = false
	sd.dohelo(arg)
}

func (sd *Smtpd) smtp_rset(args string) {
	sd.seenmail = false
	sd.out("250 flushed\r\n")
}

func (sd *Smtpd) smtp_mail(arg string) {
	if r := sd.addrparse(arg); r == 0 {
		sd.err_syntax()
		return
	}
	sd.flagbarf = !badmailfrom.Allowed(sd.addr)
	sd.seenmail = true
	sd.rcptto = sd.rcptto[:0]
	sd.mailfrom = sd.addr
	sd.out("250 ok\r\n")
}

func (sd *Smtpd) smtp_rcpt(arg string) {
	if !sd.seenmail {
		sd.err_wantmail()
		return
	}
	if r := sd.addrparse(arg); r == 0 {
		sd.err_syntax()
		return
	}
	if sd.flagbarf {
		sd.err_bmf()
		return
	}
	if sd.relayclientok {
		sd.addr += sd.relayclient
	} else {
		if !sd.addrallowed() {
			sd.err_nogateway()
			return
		}
	}
	sd.rcptto = append(sd.rcptto, sd.addr)
	sd.out("250 ok\r\n")
}

func (sd *Smtpd) put(ch byte) {
	if sd.bytestooverflow != 0 {
		sd.bytestooverflow--
		if sd.bytestooverflow == 0 {
			sd.qqt.Fail()
		}
	}
	sd.qqt.Putc(ch)
}

func (sd *Smtpd) blast() int {
	hops := 0
	state := 1
	flaginheader := true
	pos := 0           /* number of bytes since most recent \n, if fih */
	flagmaybex := true /* 1 if this line might match RECEIVED, if fih */
	flagmaybey := true /* 1 if this line might match \r\n, if fih */
	flagmaybez := true /* 1 if this line might match DELIVERED, if fih */

	for {
		ch, err := sd.ssin.ReadByte()
		if err != nil {
			if err == safeio.ErrIOTimeout {
				sd.die_alarm()
			}
			sd.die_read()
		}

		if flaginheader {
			if pos < 9 {
				if ch != "delivered"[pos] && ch != "DELIVERED"[pos] {
					flagmaybez = false
				}
				if flagmaybez && pos == 8 {
					hops++
				}
				if pos < 8 {
					if ch != "received"[pos] && ch != "RECEIVED"[pos] {
						flagmaybex = false
					}
				}
				if flagmaybex && pos == 7 {
					hops++
				}
				if pos < 2 && ch != "\r\n"[pos] {
					flagmaybey = false
				}
				if flagmaybey && pos == 1 {
					flaginheader = false
				}
			}
			pos++
			if ch == '\n' {
				pos = 0
				flagmaybex = true
				flagmaybey = true
				flagmaybez = true
			}
		}

		switch state {
		case 0:
			if ch == '\n' {
				sd.straynewline()
			}
			if ch == '\r' {
				state = 4
				continue
			}
		case 1: /* \r\n */
			if ch == '\n' {
				sd.straynewline()
			}
			if ch == '.' {
				state = 2
				continue
			}
			if ch == '\r' {
				state = 4
				continue
			}
			state = 0
		case 2: /* \r\n + . */
			if ch == '\n' {
				sd.straynewline()
			}
			if ch == '\r' {
				state = 3
				continue
			}
			state = 0
		case 3: /* \r\n + .\r */
			if ch == '\n' {
				return hops
			}
			sd.put('.')
			sd.put('\r')
			if ch == '\r' {
				state = 4
				continue
			}
			state = 0
		case 4: /* + \r */
			if ch == '\n' {
				state = 1
				break
			}
			if ch != '\r' {
				sd.put('\r')
				state = 0
			}
		}

		sd.put(ch)
	}
}

func (sd *Smtpd) acceptmessage(qp int) {
	when := time.Now()
	sd.out("250 ok ")
	sd.out(strconv.Itoa(int(when.Unix())))
	sd.out(" qt ")
	sd.out(strconv.Itoa(qp))
	sd.out("\r\n")
}

func (sd *Smtpd) smtp_data(_ string) {
	if !sd.seenmail {
		sd.err_wantmail()
		return
	}
	if len(sd.rcptto) == 0 {
		sd.err_wantrcpt()
		return
	}
	sd.seenmail = false
	if sd.databytes != 0 {
		sd.bytestooverflow = uint(sd.databytes) + 1
	}
	var err error
	if sd.qqt, err = qmail.Open(); err != nil {
		sd.err_qqt()
		return
	}
	qp := sd.qqt.Pid()
	sd.out("354 go ahead\r\n")

	sd.qqt.Received("SMTP", sd.local, sd.remoteip, sd.remotehost, sd.remoteinfo, sd.fakehelo)
	hops := sd.blast()
	too_many_hops := hops >= maxHops
	if too_many_hops {
		sd.qqt.Fail()
	}

	sd.qqt.From(sd.mailfrom)
	for _, it := range sd.rcptto {
		sd.qqt.To(it)
	}

	qqx := sd.qqt.Close()
	if qqx == "" {
		sd.acceptmessage(qp)
		return
	}
	if too_many_hops {
		sd.out("554 too many hops, this message is looping (#5.4.6)\r\n")
		return
	}
	if sd.databytes != 0 && sd.bytestooverflow == 0 {
		sd.out("552 sorry, that message size exceeds my databytes limit (#5.3.4)\r\n")
		return
	}
	if qqx[0] == 'D' {
		sd.out("554 ")
	} else {
		sd.out("451 ")
	}
	sd.out(qqx[1:])
	sd.out("\r\n")
}

func cmd_fun(fn func()) func(string) {
	return func(_ string) { fn() }
}

func (sd *Smtpd) Run(r io.Reader, w io.Writer) {
	sd.setup()
	if !ipme.Init() {
		sd.die_ipme()
	}

	sd.ssin = bufio.NewReader(safeio.NewReader(r, sd.timeout, sd.flush))
	sd.ssout = bufio.NewWriter(safeio.NewWriter(w, sd.timeout))

	sd.smtp_greet("200 ")
	sd.out(" ESMTP\r\n")

	cmds := []commands.Command{
		{"rcpt", sd.smtp_rcpt, nil},
		{"mail", sd.smtp_mail, nil},
		{"data", sd.smtp_data, sd.flush},
		{"quit", sd.smtp_quit, sd.flush},
		{"helo", sd.smtp_helo, sd.flush},
		{"ehlo", sd.smtp_ehlo, sd.flush},
		{"rset", sd.smtp_rset, nil},
		{"help", sd.smtp_help, sd.flush},
		{"noop", cmd_fun(sd.err_noop), sd.flush},
		{"vrfy", cmd_fun(sd.err_vrfy), sd.flush},
		{"", cmd_fun(sd.err_unimpl), sd.flush},
	}

	if err := commands.Loop(sd.ssin, cmds); err != nil {
		if err == safeio.ErrIOTimeout {
			sd.die_alarm()
		}
		sd.die_read()
	}
	sd.die_nomem()
}
