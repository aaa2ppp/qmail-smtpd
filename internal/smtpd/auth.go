package smtpd

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Authenticator interface {
	Authenticate(user, pass, resp string) bool
}

type authAttributes struct {
	user string
	pass string
	resp string
}

func (d *Smtpd) auth_err_input() { d.out("501 malformed auth input (#5.5.4)\r\n") }

func (d *Smtpd) auth_prompt(prompt string) {
	d.out("334 ")
	d.out(b64encode(prompt))
	d.out("\r\n")
	d.flush()
}

func (d *Smtpd) auth_gets() (string, bool) {
	s, err := d.ssin.ReadString('\n')
	if err != nil {
		d.die_read() // XXX panic!
		return "", false
	}
	s = s[:len(s)-1] // cut \n
	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	if s == "*" {
		d.out("501 auth exchange cancelled (#5.0.0)\r\n")
		return "", false
	}
	return d.auth_decode(s)
}

func (d *Smtpd) auth_decode(s string) (string, bool) {
	var ok bool
	s, ok = b64decode(s)
	if !ok {
		d.auth_err_input()
		return "", false
	}
	return s, true
}

func (d *Smtpd) auth_login(arg string) (authAttributes, bool) {
	var aa authAttributes
	var ok bool

	if arg != "" {
		if aa.user, ok = d.auth_decode(arg); !ok {
			return aa, false
		}
	} else {
		d.auth_prompt("Username:")
		if aa.user, ok = d.auth_gets(); !ok {
			return aa, false
		}
	}
	if aa.user == "" {
		d.auth_err_input()
		return aa, false
	}

	d.auth_prompt("Password:")
	if aa.pass, ok = d.auth_gets(); !ok {
		return aa, false
	}
	if aa.pass == "" {
		d.auth_err_input()
		return aa, false
	}

	return aa, true
}

func (d *Smtpd) auth_plain(arg string) (authAttributes, bool) {
	var aa authAttributes
	var slop string
	var ok bool

	if arg != "" {
		if slop, ok = d.auth_decode(arg); !ok {
			return aa, false
		}
	} else {
		d.auth_prompt("")
		if slop, ok = d.auth_gets(); !ok {
			return aa, false
		}
	}

	/* ignore authorize-id */
	i := strings.IndexByte(slop, 0)
	if i == -1 {
		d.auth_err_input()
		return aa, false
	}

	slop = slop[i+1:]
	i = strings.IndexByte(slop, 0)
	if i == -1 {
		d.auth_err_input()
		return aa, false
	}
	aa.user = slop[:i]

	slop = slop[i+1:]
	i = strings.IndexByte(slop, 0) // ???
	if i == -1 {
		i = len(slop)
	}
	aa.pass = slop[:i]

	if aa.user == "" || aa.pass == "" {
		d.auth_err_input()
		return aa, false
	}

	return aa, true
}

func cram_request(hostname string) string {
	var buf strings.Builder
	buf.WriteByte('<')
	buf.WriteString(strconv.Itoa(os.Getpid()))
	buf.WriteByte('.')
	buf.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	buf.WriteByte('@')
	buf.WriteString(hostname)
	buf.WriteByte('>')
	return buf.String()
}

func (d *Smtpd) auth_cram(arg string) (authAttributes, bool) {
	var aa authAttributes

	if arg != "" {
		d.auth_err_input()
		return aa, false
	}

	var slop string
	var ok bool

	aa.pass = cram_request(d.Hostname)
	d.auth_prompt(aa.pass)
	if slop, ok = d.auth_gets(); !ok {
		return aa, false
	}

	i := strings.IndexByte(slop, ' ')
	if i == -1 {
		d.auth_err_input()
		return aa, false
	}
	aa.user = slop[:i]

	slop = slop[i+1:]
	for len(slop) > 0 && slop[0] == ' ' {
		slop = slop[1:]
	}
	aa.resp = slop

	if aa.user == "" || aa.resp == "" {
		d.auth_err_input()
		return aa, false
	}

	return aa, true
}

func (d *Smtpd) smtp_auth(arg string) {
	if d.Auth == nil || d.Hostname == "" {
		d.out("503 auth not available (#5.3.3)\r\n")
		return
	}
	if d.authorized {
		d.out("503 you're already authenticated (#5.5.0)\r\n")
		return
	}
	if d.seenmail {
		d.out("503 no auth during mail transaction (#5.5.0)\r\n")
		return
	}

	i := strings.IndexByte(arg, ' ')
	if i == -1 {
		i = len(arg)
	}

	cmd := arg[:i]
	arg = arg[i:]
	for len(arg) > 0 && arg[0] == ' ' {
		arg = arg[1:]
	}

	var f func(string) (authAttributes, bool)
	switch strings.ToLower(cmd) {
	case "login":
		f = d.auth_login
	case "plain":
		f = d.auth_plain
	case "cram-md5":
		f = d.auth_cram
	default:
		d.out("504 auth type unimplemented (#5.5.1)\r\n")
		return
	}

	aa, ok := f(arg)
	if !ok {
		return
	}

	if !d.Auth.Authenticate(aa.user, aa.pass, aa.resp) {
		d.out("535 authorization failed (#5.7.0)\r\n")
		return
	}

	d.authorized = true
	d.RelayClient = ""
	d.RelayClientOk = true
	d.RemoteInfo = aa.user
	d.out("235 ok, go ahead (#2.0.0)\r\n")
}
