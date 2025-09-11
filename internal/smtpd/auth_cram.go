// == smtpd/auth_cram.go ==

package smtpd

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
)

// cram handles CRAM-MD5 authentication according to RFC 2195
func (h authHandler) cram(ss *session, arg string) (Credentials, error) {
	var (
		slop string
		err  error
	)

	if arg != "" {
		return nil, h.malformedInput(ss)
	}

	challenge := generateCRAMChallenge(h.cfg.Hostname)
	if err := h.challenge(ss, challenge); err != nil {
		return nil, err
	}

	if slop, err = h.response(ss); err != nil {
		return nil, err
	}

	i := strings.IndexByte(slop, ' ')
	if i == -1 {
		return nil, h.malformedInput(ss)
	}
	username, hexdigest := slop[:i], slop[i+1:]

	if username == "" || !isHexString(hexdigest, 32) {
		return nil, h.malformedInput(ss)
	}

	return &cramCredentials{
		Challenge: challenge,
		Username:  username,
		Hexdigest: hexdigest,
	}, nil
}

// generateCRAMChallenge creates a random challenge for CRAM-MD5
func generateCRAMChallenge(hostname string) string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes) // always succeeds for crypto/rand
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)
	return "<cram-" + encoded + "@" + hostname + ">"
}

// isHexString checks if a non-empty string consists only of hex characters
// If length == 0, length check is not performed
func isHexString(s string, length int) bool {
	if s == "" {
		return false
	}
	if length > 0 && len(s) != length {
		return false
	}

	for _, c := range s {
		if !isHexDigit(byte(c)) {
			return false
		}
	}
	return true
}

// isHexDigit checks if a byte is a hex digit
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'F') ||
		(c >= 'a' && c <= 'f')
}
