// == smtpd/auth_plain.go ==

package smtpd

import (
	"strings"
)

// plain handles PLAIN authentication according to RFC 4616
func (h authHandler) plain(ss *session, arg string) (Credentials, error) {
	var (
		slop string
		err  error
	)

	if arg != "" {
		if slop, err = h.decodeResponse(ss, arg); err != nil {
			return nil, err
		}
	} else {
		if err := h.challenge(ss, ""); err != nil {
			return nil, err
		}
		if slop, err = h.response(ss); err != nil {
			return nil, err
		}
	}

	parts := strings.Split(slop, "\x00")
	if len(parts) != 3 {
		return nil, h.malformedInput(ss)
	}

	authzid := parts[0] // authorization identity (identity to act as)
	authcid := parts[1] // authentication identity (identity whose password will be used)
	passwd := parts[2]  // clear-text password

	_ = authzid // ignore authorization identity

	if authcid == "" || passwd == "" {
		return nil, h.malformedInput(ss)
	}

	return &plainCredentials{
		AuthZID: authzid,
		AuthCID: authcid,
		Passwd:  passwd,
	}, nil
}
