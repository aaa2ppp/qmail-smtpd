package smtpd

import (
	"errors"
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

var ErrAuthFailed = errors.New("auth failed")

func (d *Smtpd) auth_getln() (string, error) {
	s, err := d.getln()
	if err != nil {
		return "", err
	}
	if s == "*" {
		d.out("501 auth exchange cancelled (#5.0.0)\r\n")
		return "", ErrAuthFailed
	}
	return d.auth_decode(s)
}

func (d *Smtpd) auth_decode(s string) (string, error) {
	var ok bool
	s, ok = b64decode(s)
	if !ok {
		d.auth_err_input()
		return "", ErrAuthFailed
	}
	return s, nil
}

func (d *Smtpd) auth_login(arg string) (authAttributes, error) {
	var (
		aa  authAttributes
		err error
	)

	if arg != "" {
		if aa.user, err = d.auth_decode(arg); err != nil {
			return aa, err
		}
	} else {
		d.auth_prompt("Username:")
		if aa.user, err = d.auth_getln(); err != nil {
			return aa, err
		}
	}
	if aa.user == "" {
		d.auth_err_input()
		return aa, ErrAuthFailed
	}

	d.auth_prompt("Password:")
	if aa.pass, err = d.auth_getln(); err != nil {
		return aa, err
	}
	if aa.pass == "" {
		d.auth_err_input()
		return aa, ErrAuthFailed
	}

	return aa, nil
}

func (d *Smtpd) auth_plain(arg string) (authAttributes, error) {
	var (
		aa   authAttributes
		slop string
		err  error
	)

	if arg != "" {
		if slop, err = d.auth_decode(arg); err != nil {
			return aa, err
		}
	} else {
		d.auth_prompt("")
		if slop, err = d.auth_getln(); err != nil {
			return aa, err
		}
	}

	/* ignore authorize-id */
	i := strings.IndexByte(slop, 0)
	if i == -1 {
		d.auth_err_input()
		return aa, ErrAuthFailed
	}

	slop = slop[i+1:]
	i = strings.IndexByte(slop, 0)
	if i == -1 {
		d.auth_err_input()
		return aa, ErrAuthFailed
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
		return aa, ErrAuthFailed
	}

	return aa, nil
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

func (d *Smtpd) auth_cram(arg string) (authAttributes, error) {
	var (
		aa   authAttributes
		slop string
		err  error
	)

	if arg != "" {
		d.auth_err_input()
		return aa, ErrAuthFailed
	}

	aa.pass = cram_request(d.Hostname)
	d.auth_prompt(aa.pass)
	if slop, err = d.auth_getln(); err != nil {
		return aa, ErrAuthFailed
	}

	i := strings.IndexByte(slop, ' ')
	if i == -1 {
		d.auth_err_input()
		return aa, ErrAuthFailed
	}
	aa.user = slop[:i]

	slop = slop[i+1:]
	for len(slop) > 0 && slop[0] == ' ' {
		slop = slop[1:]
	}
	aa.resp = slop

	if aa.user == "" || aa.resp == "" {
		d.auth_err_input()
		return aa, ErrAuthFailed
	}

	return aa, nil
}

func (d *Smtpd) smtp_auth(arg string) error {
	if d.Auth == nil || d.Hostname == "" {
		return d.out("503 auth not available (#5.3.3)\r\n")
	}
	if d.authorized {
		return d.out("503 you're already authenticated (#5.5.0)\r\n")
	}
	if d.seenmail {
		return d.out("503 no auth during mail transaction (#5.5.0)\r\n")
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

	var f func(string) (authAttributes, error)
	switch strings.ToLower(cmd) {
	case "login":
		if !d.tlsEnabled {
			return d.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		f = d.auth_login
	case "plain":
		if !d.tlsEnabled {
			return d.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		f = d.auth_plain
	case "cram-md5":
		f = d.auth_cram
	default:
		return d.out("504 auth type unimplemented (#5.5.1)\r\n")
	}

	aa, err := f(arg)
	if err != nil {
		if errors.Is(err, ErrAuthFailed) {
			return nil
		}
		return err
	}

	if !d.Auth.Authenticate(aa.user, aa.pass, aa.resp) {
		return d.out("535 authorization failed (#5.7.0)\r\n")
	}

	d.authorized = true
	d.RelayClient = ""
	d.RelayClientOk = true
	d.RemoteInfo = aa.user
	return d.out("235 ok, go ahead (#2.0.0)\r\n")
}
