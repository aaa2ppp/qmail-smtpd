package smtpd

import (
	"cmp"
	"errors"
	"log"
	"strings"
)

// SMTP AUTH implementation according to RFC 4954
// Supported mechanisms:
// - PLAIN (RFC 4616)
// - CRAM-MD5 (RFC 2195)
// - LOGIN (non-standard, for backward compatibility)

// NOTE: Consider moving to interfaces/ package in a separate refactoring.
// This change should be atomic and focused solely on interface relocation.
type Authenticator interface {
	Authenticate(cred Credentials) (authResult, error)
}

// errAuthRejected internal signal error
var errAuthRejected = errors.New("auth malformed or canceled")

// authHandler handles SMTP authentication using various mechanisms
type authHandler struct {
	cfg *Config
}

// malformedInput sends 501 to client, always returns errAuthRejected or IO error
func (h authHandler) malformedInput(ss *session) error {
	return cmp.Or(ss.out("501 malformed auth input (#5.5.4)\r\n"), errAuthRejected)
}

func (h authHandler) challenge(ss *session, challenge string) error {
	_ = ss.out("334 ")
	_ = ss.out(b64encode(challenge))
	_ = ss.out("\r\n")
	return ss.Flush()
}

func (h authHandler) response(ss *session) (string, error) {
	s, err := ss.ReadLine()
	if err != nil {
		return "", err
	}
	if s == "*" {
		return "", cmp.Or(ss.out("501 auth exchange cancelled (#5.0.0)\r\n"), errAuthRejected)
	}
	return h.decodeResponse(ss, s)
}

func (h authHandler) decodeResponse(ss *session, s string) (string, error) {
	decoded, ok := b64decode(s)
	if !ok {
		return "", h.malformedInput(ss)
	}
	return decoded, nil
}

// smtp_auth handles SMTP AUTH command
func (d *Server) smtp_auth(ss *session, arg string) error {
	auth := authHandler{cfg: d.cfg}

	// Preliminary checks
	if d.cfg.Auth == nil || d.cfg.AuthFQDN == "" {
		return ss.out("503 auth not available (#5.3.3)\r\n")
	}
	if ss.authorized {
		return ss.out("503 you're already authenticated (#5.5.0)\r\n")
	}
	if ss.seenmail {
		return ss.out("503 no auth during mail transaction (#5.5.0)\r\n")
	}

	mechanism, arg := parseCmdLine(arg)

	// Select authentication mechanism
	var (
		cred Credentials
		err  error
	)

	switch strings.ToLower(mechanism) {
	case "login":
		if !ss.tlsEnabled {
			return ss.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		cred, err = auth.login(ss, arg)
	case "plain":
		if !ss.tlsEnabled {
			return ss.out("504 auth type unimplemented (#5.5.1)\r\n")
		}
		cred, err = auth.plain(ss, arg)
	case "cram-md5":
		cred, err = auth.cram(ss, arg)
	default:
		return ss.out("504 auth type unimplemented (#5.5.1)\r\n")
	}

	if err != nil {
		if errors.Is(err, errAuthRejected) {
			return nil // Error already sent to client
		}
		return err
	}

	res, err := d.cfg.Auth.Authenticate(cred)

	if err != nil {
		// Internal server error - log the details but don't expose to client
		log.Printf("Authentication backend error: %v", err)
		return ss.out("454 temporary authentication failure (#4.7.0)\r\n")
	}
	if !res.Success {
		// Authentication failed - permanent failure
		return ss.out("535 authentication credentials invalid (#5.7.0)\r\n")
	}

	d.setAuthorized(ss, res.Username)
	return ss.out("235 ok, go ahead (#2.0.0)\r\n")
}

func (d *Server) setAuthorized(ss *session, username string) {
	ss.authorized = true
	ss.user = username
	ss.env.Set("TCPREMOTEINFO", username)
	ss.relayclient = ""
	ss.relayclientok = true
}

func (d *Server) resetAuthorized(ss *session) {
	ss.authorized = false
	ss.user = ""
	ss.env.Unset("TCPREMOTEINFO")
	ss.relayclient, ss.relayclientok = ss.env.Lookup("RELAYCLIENT")
}
