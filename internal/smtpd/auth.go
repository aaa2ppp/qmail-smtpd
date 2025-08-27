package smtpd

import (
	"cmp"
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

func (d *Smtpd) auth_err_input(ss *Session) error {
	return ss.out("501 malformed auth input (#5.5.4)\r\n")
}

func (d *Smtpd) auth_prompt(ss *Session, prompt string) error {
	_ = ss.out("334 ")
	_ = ss.out(b64encode(prompt))
	_ = ss.out("\r\n")
	return ss.flush()
}

var ErrAuthFailed = errors.New("auth failed")

func (d *Smtpd) auth_getln(ss *Session) (string, error) {
	s, err := ss.getln()
	if err != nil {
		return "", err
	}
	if s == "*" {
		return "", cmp.Or(ss.out("501 auth exchange cancelled (#5.0.0)\r\n"), ErrAuthFailed)
	}
	return d.auth_decode(ss, s)
}

func (d *Smtpd) auth_decode(ss *Session, s string) (string, error) {
	var ok bool
	s, ok = b64decode(s)
	if !ok {
		return "", cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}
	return s, nil
}

func (d *Smtpd) auth_login(ss *Session, arg string) (authAttributes, error) {
	var (
		aa  authAttributes
		err error
	)

	if arg != "" {
		if aa.user, err = d.auth_decode(ss, arg); err != nil {
			return aa, err
		}
	} else {
		d.auth_prompt(ss, "Username:")
		if aa.user, err = d.auth_getln(ss); err != nil {
			return aa, err
		}
	}
	if aa.user == "" {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}

	d.auth_prompt(ss, "Password:")
	if aa.pass, err = d.auth_getln(ss); err != nil {
		return aa, err
	}
	if aa.pass == "" {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}

	return aa, nil
}

func (d *Smtpd) auth_plain(ss *Session, arg string) (authAttributes, error) {
	var (
		aa   authAttributes
		slop string
		err  error
	)

	if arg != "" {
		if slop, err = d.auth_decode(ss, arg); err != nil {
			return aa, err
		}
	} else {
		if err := d.auth_prompt(ss, ""); err != nil {
			return aa, err
		}
		if slop, err = d.auth_getln(ss); err != nil {
			return aa, err
		}
	}

	/* ignore authorize-id */
	i := strings.IndexByte(slop, 0)
	if i == -1 {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}

	slop = slop[i+1:]
	i = strings.IndexByte(slop, 0)
	if i == -1 {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}
	aa.user = slop[:i]

	slop = slop[i+1:]
	i = strings.IndexByte(slop, 0) // ???
	if i == -1 {
		i = len(slop)
	}
	aa.pass = slop[:i]

	if aa.user == "" || aa.pass == "" {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
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

func (d *Smtpd) auth_cram(ss *Session, arg string) (authAttributes, error) {
	var (
		aa   authAttributes
		slop string
		err  error
	)

	if arg != "" {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}

	aa.pass = cram_request(d.cfg.Hostname)
	if err := d.auth_prompt(ss, aa.pass); err != nil {
		return aa, err
	}
	if slop, err = d.auth_getln(ss); err != nil {
		return aa, err
	}

	i := strings.IndexByte(slop, ' ')
	if i == -1 {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}
	aa.user = slop[:i]

	slop = slop[i+1:]
	for len(slop) > 0 && slop[0] == ' ' {
		slop = slop[1:]
	}
	aa.resp = slop

	if aa.user == "" || aa.resp == "" {
		return aa, cmp.Or(d.auth_err_input(ss), ErrAuthFailed)
	}

	return aa, nil
}

func (d *Smtpd) smtp_auth(ss *Session, arg string) error {
	if d.cfg.Auth == nil || d.cfg.Hostname == "" {
		return ss.out("503 auth not available (#5.3.3)\r\n")
	}
	if ss.authorized {
		return ss.out("503 you're already authenticated (#5.5.0)\r\n")
	}
	if ss.seenmail {
		return ss.out("503 no auth during mail transaction (#5.5.0)\r\n")
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

	var authFn func(ss *Session, arg string) (authAttributes, error)
	switch strings.ToLower(cmd) {
	case "login":
		if !ss.tlsEnabled {
			return ss.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		authFn = d.auth_login
	case "plain":
		if !ss.tlsEnabled {
			return ss.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		authFn = d.auth_plain
	case "cram-md5":
		authFn = d.auth_cram
	default:
		return ss.out("504 auth type unimplemented (#5.5.1)\r\n")
	}

	aa, err := authFn(ss, arg)
	if err != nil {
		if errors.Is(err, ErrAuthFailed) {
			return nil
		}
		return err
	}

	if !d.cfg.Auth.Authenticate(aa.user, aa.pass, aa.resp) {
		return ss.out("535 authorization failed (#5.7.0)\r\n")
	}

	d.setAuthorized(ss, aa.user)
	return ss.out("235 ok, go ahead (#2.0.0)\r\n")
}

func (d *Smtpd) setAuthorized(ss *Session, user string) {
	ss.authorized = true
	ss.remoteInfo = user
	ss.relayClient = ""
	ss.relayClientOk = true
}

func (d *Smtpd) resetAuthorized(ss *Session) {
	ss.authorized = false
	ss.remoteInfo = ""
	ss.relayClient = d.cfg.RelayClient
	ss.relayClientOk = d.cfg.RelayClientOk
}
