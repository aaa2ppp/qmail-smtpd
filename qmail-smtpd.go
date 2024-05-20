package main

import (
	"bufio"
	"bytes"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const MAXHOPS = 100

var databytes = 0
var timeout = 1200 * time.Second // WTF: why so many?

type safeWriter os.File

func (r *safeWriter) Write(b []byte) (n int, err error) {
	// we will be died when the timeout expires, so there is no point in buffering the channel
	done := make(chan struct{})
	go func() {
		n, err = (*os.File)(r).Write(b)
		done <- struct{}{}
	}()

	tm := time.NewTimer(timeout)
	select {
	case <-tm.C:
		_exit(1)
		return 0, errors.New("write timeout") // _exit never returns so this will never happen
	case <-done:
		tm.Stop()
		return n, err
	}
}

var ssout = bufio.NewWriter((*safeWriter)(os.Stdout))

func flush() {
	if err := ssout.Flush(); err != nil {
		log.Fatal(err)
	}
}

func out(s string) {
	if _, err := ssout.WriteString(s); err != nil {
		log.Fatal(err)
	}
}

func die_read()     { _exit(1) }
func die_alarm()    { out("451 timeout (#4.4.2)\r\n"); flush(); _exit(1) }
func die_nomem()    { out("421 out of memory (#4.3.0)\r\n"); flush(); _exit(1) }
func die_control()  { out("421 unable to read controls (#4.3.0)\r\n"); flush(); _exit(1) }
func die_ipme()     { out("421 unable to figure out my IP addresses (#4.3.0)\r\n"); flush(); _exit(1) }
func straynewline() { out("451 See http://pobox.com/~djb/docs/smtplf.html.\r\n"); flush(); _exit(1) }

func err_bmf() { out("553 sorry, your envelope sender is in my badmailfrom list (#5.7.1)\r\n") }
func err_nogateway() {
	out("553 sorry, that domain isn't in my list of allowed rcpthosts (#5.7.1)\r\n")
}
func err_unimpl()   { out("502 unimplemented (#5.5.1)\r\n") }
func err_syntax()   { out("555 syntax error (#5.5.4)\r\n") }
func err_wantmail() { out("503 MAIL first (#5.5.1)\r\n") }
func err_wantrcpt() { out("503 RCPT first (#5.5.1)\r\n") }
func err_noop()     { out("250 ok\r\n") }
func err_vrfy()     { out("252 send some mail, i'll try my best\r\n") }
func err_qqt()      { out("451 qqt failure (#4.3.0)\r\n") }

var greeting string

func smtp_greet(code string) {
	out(code)
	out(greeting)
}

func smtp_help(_ string) {
	out("214 qmail home page: http://pobox.com/~djb/qmail.html\r\n")
}

func smtp_quit(_ string) {
	smtp_greet("221 ")
	out("\r\n")
	flush()
	_exit(0)
}

var remoteip string
var remotehost string
var remoteinfo string
var local string
var relayclient string
var relayclientok bool

var helohost string
var fakehelo string /* pointer into helohost, or 0 */

func dohelo(arg string) {
	helohost = arg
	if case_diffs(remotehost, helohost) {
		fakehelo = helohost
	}
}

var liphostok bool
var liphost string
var bmfok bool
var mapbmf = tConstmap{}

func setup() {
	if control_init() == -1 {
		die_control()
	}

	if s, r := control_rldef("control/smtpgreeting", true, ""); r != 1 {
		die_control()
	} else {
		greeting = s
	}

	if s, r := control_rldef("control/localiphost", true, ""); r == -1 {
		die_control()
	} else if r == 1 {
		liphost = s
		liphostok = true
	}

	if i, r := control_readint("control/timeoutsmtpd"); r == -1 {
		die_control()
	} else if r == 1 {
		if i <= 0 {
			i = 1
		}
		timeout = time.Duration(i) * time.Second
	}

	if rcpthosts_init() == -1 {
		die_control()
	}

	if ss, r := control_readfile("control/badmailfrom", false); r == -1 {
		die_control()
	} else if r == 1 {
		constmap_init(mapbmf, ss)
		bmfok = true
	}

	if i, r := control_readint("control/databytes"); r == -1 {
		die_control()
	} else if r == 1 {
		databytes = i
	}

	// x = env_get("DATABYTES");
	// if (x) { scan_ulong(x,&u); databytes = u; }
	// if (!(databytes + 1)) --databytes;  // WTF: if databytes == -1 then databytes = -2 ?
	if x := os.Getenv("DATABYTES"); x != "" {
		databytes, _ = strconv.Atoi(x)
	}
	if databytes+1 == 0 { // WTF?
		databytes--
	}

	remoteip = os.Getenv("TCPREMOTEIP")
	if remoteip == "" {
		remoteip = "unknown"
	}

	local = os.Getenv("TCPLOCALHOST")
	if local == "" {
		local = os.Getenv("TCPLOCALIP")
	}
	if local == "" {
		local = "unknown"
	}

	remotehost = os.Getenv("TCPREMOTEHOST")
	if remotehost == "" {
		remotehost = "unknown"
	}

	remoteinfo = os.Getenv("TCPREMOTEINFO")
	relayclient, relayclientok = os.LookupEnv("RELAYCLIENT")

	dohelo(remotehost)
}

var addr string

func addrparse(arg string) int {
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

	var addrbuf []byte
	var flagesc bool
	var flagquoted bool

	for _, ch := range []byte(arg) { /* copy arg to addr, stripping quotes */
		if flagesc {
			addrbuf = append(addrbuf, ch)
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
				addrbuf = append(addrbuf, ch)
			}
		}
	}
	/* could check for termination failure here, but why bother? */

	if liphostok {
		i := bytes.LastIndexByte(addrbuf, '@')
		if i != -1 { /* if not, partner should go read rfc 821 */
			if i+1 < len(addrbuf) && addrbuf[i+1] == '[' {
				l, ip := ip_scanbracket(unsafeString(addrbuf[i+1:]))
				if i+1+l == len(addrbuf) {
					if ipme_is(ip) {
						addrbuf = append(addrbuf[:i+1], liphost...)
					}
				}
			}
		}
	}

	if len(addrbuf) > 900 {
		return 0
	}

	addr = string(addrbuf)
	return 1
}

func bmfcheck() bool {
	if !bmfok {
		return false
	}
	if constmap(mapbmf, addr) {
		return true
	}
	if j := strings.IndexByte(addr, '@'); j != -1 {
		if constmap(mapbmf, addr[j+1:]) {
			return true
		}
	}
	return false
}

func addrallowed() bool {
	r := rcpthosts(addr)
	if r == -1 {
		die_control()
	}
	return r != 0
}

var seenmail bool
var flagbarf bool /* defined if seenmail */
var mailfrom string
var rcptto []string

func smtp_helo(arg string) {
	smtp_greet("250 ")
	out("\r\n")
	seenmail = false
	dohelo(arg)
}

func smtp_ehlo(arg string) {
	smtp_greet("250-")
	out("\r\n250-PIPELINING\r\n250 8BITMIME\r\n")
	seenmail = false
	dohelo(arg)
}

func smtp_rset(args string) {
	seenmail = false
	out("250 flushed\r\n")
}

func smtp_mail(arg string) {
	if r := addrparse(arg); r == 0 {
		err_syntax()
		return
	}
	flagbarf = bmfcheck()
	seenmail = true
	rcptto = rcptto[:0]
	mailfrom = addr
	out("250 ok\r\n")
}

func smtp_rcpt(arg string) {
	if !seenmail {
		err_wantmail()
		return
	}
	if r := addrparse(arg); r == 0 {
		err_syntax()
		return
	}
	if flagbarf {
		err_bmf()
		return
	}
	if relayclientok {
		addr += relayclient
	} else {
		if !addrallowed() {
			err_nogateway()
			return
		}
	}
	rcptto = append(rcptto, addr)
	out("250 ok\r\n")
}

type safeReader os.File

func (r *safeReader) Read(b []byte) (n int, err error) {
	flush()

	// we will be died when the timeout expires, so there is no point in buffering the channel
	done := make(chan struct{})
	go func() {
		n, err = (*os.File)(r).Read(b)
		done <- struct{}{}
	}()

	tm := time.NewTimer(timeout)
	select {
	case <-tm.C:
		die_alarm()
		return 0, errors.New("read timeout") // die_* never returns so this will never happen
	case <-done:
		tm.Stop()
		return n, err
	}
}

var ssin = bufio.NewReader((*safeReader)(os.Stdin))

var qqt tQmail
var bytestooverflow uint

func put(ch byte) {
	if bytestooverflow != 0 {
		bytestooverflow--
		if bytestooverflow == 0 {
			qmail_fail(&qqt)
		}
	}
	qmail_putc(&qqt, ch)
}

func blast() int {
	hops := 0
	state := 1
	flaginheader := true
	pos := 0           /* number of bytes since most recent \n, if fih */
	flagmaybex := true /* 1 if this line might match RECEIVED, if fih */
	flagmaybey := true /* 1 if this line might match \r\n, if fih */
	flagmaybez := true /* 1 if this line might match DELIVERED, if fih */

	for {
		ch, _ := ssin.ReadByte()

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
				straynewline()
			}
			if ch == '\r' {
				state = 4
				continue
			}
		case 1: /* \r\n */
			if ch == '\n' {
				straynewline()
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
				straynewline()
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
			put('.')
			put('\r')
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
				put('\r')
				state = 0
			}
		}

		put(ch)
	}
}

func acceptmessage(qp int) {
	when := time.Now()
	out("250 ok ")
	out(strconv.Itoa(int(when.Unix())))
	out(" qt ")
	out(strconv.Itoa(qp))
	out("\r\n")
}

func smtp_data(_ string) {
	if !seenmail {
		err_wantmail()
		return
	}
	if len(rcptto) == 0 {
		err_wantrcpt()
		return
	}
	seenmail = false
	if databytes != 0 {
		bytestooverflow = uint(databytes) + 1
	}
	if qmail_open(&qqt) == -1 {
		err_qqt()
		return
	}
	qp := qmail_qp(&qqt)
	out("354 go ahead\r\n")

	received(&qqt, "SMTP", local, remoteip, remotehost, remoteinfo, fakehelo)
	hops := blast()
	too_many_hops := hops >= MAXHOPS
	if too_many_hops {
		qmail_fail(&qqt)
	}
	
	qmail_from(&qqt, mailfrom)
	for _, it := range rcptto {
		qmail_to(&qqt, it)
	}

	qqx := qmail_close(&qqt)
	if qqx == "" {
		acceptmessage(qp)
		return
	}
	if too_many_hops {
		out("554 too many hops, this message is looping (#5.4.6)\r\n")
		return
	}
	if databytes != 0 && bytestooverflow == 0 {
		out("552 sorry, that message size exceeds my databytes limit (#5.3.4)\r\n")
		return
	}
	if qqx[0] == 'D' {
		out("554 ")
	} else {
		out("451 ")
	}
	out(qqx[1:])
	out("\r\n")
}

func cmd_fun(fn func()) func(string) {
	return func(_ string) { fn() }
}

var smtpcommands = []tCommands{
	{"rcpt", smtp_rcpt, nil},
	{"mail", smtp_mail, nil},
	{"data", smtp_data, flush},
	{"quit", smtp_quit, flush},
	{"helo", smtp_helo, flush},
	{"ehlo", smtp_ehlo, flush},
	{"rset", smtp_rset, nil},
	{"help", smtp_help, flush},
	{"noop", cmd_fun(err_noop), flush},
	{"vrfy", cmd_fun(err_vrfy), flush},
	{"", cmd_fun(err_unimpl), flush},
}

func main() {
	sig_pipeignore()
	if err := os.Chdir(auto_qmail); err != nil {
		log.Fatal(err)
	}
	setup()
	if !ipme_init() {
		die_ipme()
	}
	smtp_greet("200 ")
	out(" ESMTP\r\n")
	if commands(ssin, smtpcommands) == 0 {
		die_read()
	}
	die_nomem()
}
