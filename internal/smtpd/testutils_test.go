package smtpd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"time"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/todo/scan"
)

/*
type alwaysAuth struct{}

func (a alwaysAuth) Authenticate(_, _, _ string) bool { return true }

type alwaysNoAuth struct{}

func (a alwaysNoAuth) Auth(_, _, _ string) bool { return false }
*/

type fakeAuth struct {
	ok bool
}

func (a *fakeAuth) Authenticate(cred Credentials) (authResult, error) {
	return authResult{
		Username: "Fake-Name",
		Success:  a.ok,
	}, nil
}

var _ Authenticator = &fakeAuth{}

type alwaysMatch struct{}

func (m alwaysMatch) Match(_ string) bool { return true }

type alwaysNotMatch struct{}

func (m alwaysNotMatch) Match(_ string) bool { return false }

type fakeReader struct {
	*strings.Reader
	timeout   time.Duration
	_deadline time.Time
	readErr   error
	closeErr  error
}

func newFakeReader(s string) *fakeReader {
	return &fakeReader{
		Reader: strings.NewReader(s),
	}
}

func newFakeReaderWithTimeout(s string, timeout time.Duration) *fakeReader {
	return &fakeReader{
		Reader:  strings.NewReader(s),
		timeout: timeout,
	}
}

func newFakeReaderAlwaysErr() *fakeReader {
	return &fakeReader{
		readErr:  errors.New("read error"),
		closeErr: errors.New("close error"),
	}
}

func (r *fakeReader) SetReadDeadline(t time.Time) error {
	r._deadline = t
	return nil
}

func (r *fakeReader) Close() error {
	return nil
}

func (r *fakeReader) Read(b []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	// TODO: remove deadline logic? — seems to belong to safeio tests, not handlers.
	// If needed elsewhere (e.g. session timeout tests), keep but document usage.
	if !r._deadline.Equal(time.Time{}) && r._deadline.Before(time.Now().Add(r.timeout)) {
		return 0, os.ErrDeadlineExceeded
	}
	return r.Reader.Read(b)
}

type fakeWriter struct {
	bytes.Buffer
	timeout   time.Duration
	_deadline time.Time
	writeErr  error
	closeErr  error
}

func (w *fakeWriter) SetWriteDeadline(t time.Time) error {
	w._deadline = t
	return nil
}

func (w *fakeWriter) Close() error {
	if w.closeErr != nil {
		return w.closeErr
	}
	return nil
}

func (w *fakeWriter) Write(b []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	// TODO: remove deadline logic? — seems to belong to safeio tests, not handlers.
	// If needed elsewhere (e.g. session timeout tests), keep but document usage.
	if !w._deadline.Equal(time.Time{}) && w._deadline.Before(time.Now().Add(w.timeout)) {
		return 0, os.ErrDeadlineExceeded
	}
	return w.Buffer.Write(b)
}

type fakeQueue struct {
	pid        int
	beginErr   error
	writeErr   error
	_stickyErr error
	commitErr  error
}

func (qq *fakeQueue) Begin(mailForm string, rcptTo []string, env env.Env) (Queue, error) {
	if qq.beginErr != nil {
		return nil, qq.beginErr
	}
	return qq, nil
}

func (qq *fakeQueue) Pid() int { return qq.pid }

func (qq *fakeQueue) WriteByte(_ byte) error {
	if qq._stickyErr == nil && qq.writeErr != nil {
		qq._stickyErr = qq.writeErr
	}
	return qq._stickyErr
}

func (qq *fakeQueue) WriteString(s string) (int, error) {
	if qq._stickyErr == nil && qq.writeErr != nil {
		qq._stickyErr = qq.writeErr
	}
	return len(s), qq._stickyErr
}

func (qq *fakeQueue) Commit() error {
	if qq._stickyErr == nil && qq.commitErr != nil {
		qq._stickyErr = qq.commitErr
	}
	if qq._stickyErr != nil {
		return qq._stickyErr
	}
	qq._stickyErr = qmail.ErrTxDone
	return nil
}

func (qq *fakeQueue) Rollback() error {
	if qq._stickyErr != nil {
		return qq._stickyErr
	}
	qq._stickyErr = qmail.ErrTxDone
	return nil
}

var (
	_ Qmail = (*fakeQueue)(nil)
	_ Queue = (*fakeQueue)(nil)
)

func extractCodes(answers string) []int {
	const (
		crlf      = "\r\n"
		errSignal = -1
	)

	codes := []int{}
	for pos := 0; pos < len(answers); {
		offset := strings.Index(answers[pos:], crlf)
		if offset == -1 || offset < 4 { // "250 ...\r\n"
			codes = append(codes, errSignal)
			break
		}

		n, code := scan.ScanUlong(answers[pos:])
		if n != 3 {
			codes = append(codes, errSignal)
		} else if answers[pos+3] == '-' {
			// skip
		} else if answers[pos+3] == ' ' {
			codes = append(codes, int(code))
		} else {
			codes = append(codes, errSignal)
		}

		pos += offset + len(crlf)
	}

	return codes
}

func addCr(s string) string {
	a := strings.Split(s, "\n")
	for i := range a {
		if n := len(a[i]); n > 0 && a[i][n-1] == '\r' {
			a[i] = a[i][:n-1]
		}
	}
	return strings.Join(a, "\r\n")
}

func delCr(s string) string {
	a := strings.Split(s, "\n")
	for i := range a {
		if n := len(a[i]); n > 0 && a[i][n-1] == '\r' {
			a[i] = a[i][:n-1]
		}
	}
	return strings.Join(a, "\n")
}

// checkMsgID validates that s is a valid msg-id per RFC 5322.
// It checks the format: <local@domain>, where local and domain are dot-atoms.
// This is used to validate CRAM-MD5 challenges.
func checkMsgID(s string) bool {
	// Must start with '<' and end with '>'
	if len(s) < 3 || s[0] != '<' || s[len(s)-1] != '>' {
		return false
	}

	// Extract inner: local@domain
	inner := s[1 : len(s)-1]

	// Must contain exactly one '@'
	at := strings.LastIndex(inner, "@")
	if at == -1 || at == 0 || at == len(inner)-1 {
		return false
	}

	local := inner[:at]
	domain := inner[at+1:]

	// Both local and domain must be valid dot-atoms
	return checkDotAtom(local) && checkDotAtom(domain)
}

// checkDotAtom validates that s is a valid "dot-atom" per RFC 5322.
// It must be non-empty, not start/end with '.', and contain only allowed chars.
func checkDotAtom(s string) bool {
	const allowSymbols = "!#$%&'*+-/=?^_`{|}~"

	if len(s) == 0 {
		return false
	}

	mayBeDot := false
	for _, c := range []byte(s) {
		switch {
		case 'a' <= c && c <= 'z',
			'A' <= c && c <= 'Z',
			'0' <= c && c <= '9',
			strings.IndexByte(allowSymbols, c) != -1:
			mayBeDot = true
		case c == '.':
			if !mayBeDot {
				return false
			}
			mayBeDot = false
		default:
			return false
		}
	}

	// Last char must not be '.'
	return mayBeDot
}
