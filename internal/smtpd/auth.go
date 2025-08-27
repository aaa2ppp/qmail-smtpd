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

func auth_err_input(d *Smtpd) { d.out("501 malformed auth input (#5.5.4)\r\n") }

func auth_prompt(d *Smtpd, prompt string) {
	d.out("334 ")
	d.out(b64encode(prompt))
	d.out("\r\n")
	d.flush()
}

var ErrAuthFailed = errors.New("auth failed")

func auth_getln(d *Smtpd) (string, error) {
	s, err := d.getln()
	if err != nil {
		return "", err
	}
	if s == "*" {
		d.out("501 auth exchange cancelled (#5.0.0)\r\n")
		return "", ErrAuthFailed
	}
	return auth_decode(d, s)
}

func auth_decode(d *Smtpd, s string) (string, error) {
	var ok bool
	s, ok = b64decode(s)
	if !ok {
		auth_err_input(d)
		return "", ErrAuthFailed
	}
	return s, nil
}

func auth_login(d *Smtpd, arg string) (authAttributes, error) {
	var (
		aa  authAttributes
		err error
	)

	if arg != "" {
		if aa.user, err = auth_decode(d, arg); err != nil {
			return aa, err
		}
	} else {
		auth_prompt(d, "Username:")
		if aa.user, err = auth_getln(d); err != nil {
			return aa, err
		}
	}
	if aa.user == "" {
		auth_err_input(d)
		return aa, ErrAuthFailed
	}

	auth_prompt(d, "Password:")
	if aa.pass, err = auth_getln(d); err != nil {
		return aa, err
	}
	if aa.pass == "" {
		auth_err_input(d)
		return aa, ErrAuthFailed
	}

	return aa, nil
}

func auth_plain(d *Smtpd, arg string) (authAttributes, error) {
	var (
		aa   authAttributes
		slop string
		err  error
	)

	if arg != "" {
		if slop, err = auth_decode(d, arg); err != nil {
			return aa, err
		}
	} else {
		auth_prompt(d, "")
		if slop, err = auth_getln(d); err != nil {
			return aa, err
		}
	}

	/* ignore authorize-id */
	i := strings.IndexByte(slop, 0)
	if i == -1 {
		auth_err_input(d)
		return aa, ErrAuthFailed
	}

	slop = slop[i+1:]
	i = strings.IndexByte(slop, 0)
	if i == -1 {
		auth_err_input(d)
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
		auth_err_input(d)
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

func auth_cram(d *Smtpd, arg string) (authAttributes, error) {
	var (
		aa   authAttributes
		slop string
		err  error
	)

	if arg != "" {
		auth_err_input(d)
		return aa, ErrAuthFailed
	}

	aa.pass = cram_request(d.Hostname)
	auth_prompt(d, aa.pass)
	if slop, err = auth_getln(d); err != nil {
		return aa, ErrAuthFailed
	}

	i := strings.IndexByte(slop, ' ')
	if i == -1 {
		auth_err_input(d)
		return aa, ErrAuthFailed
	}
	aa.user = slop[:i]

	slop = slop[i+1:]
	for len(slop) > 0 && slop[0] == ' ' {
		slop = slop[1:]
	}
	aa.resp = slop

	if aa.user == "" || aa.resp == "" {
		auth_err_input(d)
		return aa, ErrAuthFailed
	}

	return aa, nil
}

func smtp_auth(d *Smtpd, arg string) error {
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

	var fn func(d *Smtpd, arg string) (authAttributes, error)
	switch strings.ToLower(cmd) {
	case "login":
		if !d.tlsEnabled {
			return d.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		fn = auth_login
	case "plain":
		if !d.tlsEnabled {
			return d.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		fn = auth_plain
	case "cram-md5":
		fn = auth_cram
	default:
		return d.out("504 auth type unimplemented (#5.5.1)\r\n")
	}

	aa, err := fn(d, arg)
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
