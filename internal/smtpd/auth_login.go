// == smtpd/auth_login.go ==

package smtpd

// login handles non-standard LOGIN mechanism (for backward compatibility)
func (h authHandler) login(ss *session, arg string) (Credentials, error) {
	var (
		username string
		password string
		err      error
	)

	if arg != "" {
		if username, err = h.decodeResponse(ss, arg); err != nil {
			return nil, err
		}
	} else {
		h.challenge(ss, "Username:")
		if username, err = h.response(ss); err != nil {
			return nil, err
		}
	}
	if username == "" {
		return nil, h.malformedInput(ss)
	}

	h.challenge(ss, "Password:")
	if password, err = h.response(ss); err != nil {
		return nil, err
	}
	if password == "" {
		return nil, h.malformedInput(ss)
	}

	return &plainCredentials{
		AuthZID: username,
		AuthCID: username,
		Passwd:  password,
	}, nil
}
