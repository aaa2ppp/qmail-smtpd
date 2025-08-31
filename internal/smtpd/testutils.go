package smtpd

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"time"

	"qmail-smtpd/internal/pipeconn"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/scan"
)

type alwaysAuth struct{}

func (a alwaysAuth) Authenticate(_, _, _ string) bool { return true }

type alwaysNoAuth struct{}

func (a alwaysNoAuth) Auth(_, _, _ string) bool { return false }

type alwaysMatch struct{}

func (m alwaysMatch) Match(_ string) bool { return true }

type alwaysNotMatch struct{}

func (m alwaysNotMatch) Match(_ string) bool { return false }

type fakeReader struct {
	*strings.Reader
	timeout   time.Duration
	deadline  time.Time
	alwaysErr bool
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
		alwaysErr: true,
	}
}

func (r *fakeReader) SetReadDeadline(t time.Time) error {
	r.deadline = t
	return nil
}

func (r *fakeReader) Close() error {
	return nil
}

func (r *fakeReader) Read(b []byte) (int, error) {
	if r.alwaysErr {
		return 0, errors.New("read error")
	}
	if !r.deadline.Equal(time.Time{}) && r.deadline.Before(time.Now().Add(r.timeout)) {
		time.Sleep(r.timeout)
		return 0, os.ErrDeadlineExceeded
	}
	return r.Reader.Read(b)
}

type fakeWriter struct {
	bytes.Buffer
	timeout  time.Duration
	deadline time.Time
}

func (w *fakeWriter) SetWriteDeadline(t time.Time) error {
	w.deadline = t
	return nil
}

func (w *fakeWriter) Close() error {
	return nil
}

func (w *fakeWriter) Write(b []byte) (int, error) {
	if !w.deadline.Equal(time.Time{}) && w.deadline.Before(time.Now().Add(w.timeout)) {
		time.Sleep(w.timeout)
		return 0, os.ErrDeadlineExceeded
	}
	return w.Buffer.Write(b)
}

type fakeQueue struct {
	result string
}

func (qq *fakeQueue) Begin(mailForm string, rcptTo []string, opts qmail.Env) (Queue, error) {
	return qq, nil
}

func (qq *fakeQueue) Pid() int                          { return 7777 }
func (qq *fakeQueue) WriteByte(_ byte) error            { return nil }
func (qq *fakeQueue) WriteString(s string) (int, error) { return len(s), nil }
func (qq *fakeQueue) Commit() error                     { return nil }
func (qq *fakeQueue) Rollback() error                   { return nil }

var (
	_ Qmail = (*fakeQueue)(nil)
	_ Queue = (*fakeQueue)(nil)
)

func extractCodes(answers string) []int {
	crln := "\r\n"
	answers = strings.TrimSuffix(answers, crln)
	lines := strings.Split(answers, crln)
	codes := make([]int, 0, len(lines))
	for _, line := range lines {
		j, code := scan.ScanUlong(line)
		if j < len(line) && line[j] == '-' {
			continue
		}
		codes = append(codes, int(code))
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

func createServerAndSession(cfg *Config, state sessionState) (*Server, *session, *fakeWriter) {
	r := newFakeReader("") // stub
	w := &fakeWriter{}
	srv, ss := createServerAndSessionWithRW(cfg, state, r, w)
	return srv, ss, w
}

func createServerAndSessionWithRW(cfg *Config, state sessionState, r *fakeReader, w *fakeWriter) (*Server, *session) {
	srv := NewServer(cfg)
	conn := &pipeconn.Conn{
		Reader: r,
		Writer: w,
	}
	ss := srv.newSession(conn, qmail.Env{})
	if !reflect.DeepEqual(state, sessionState{}) {
		ss.sessionState = state
	}
	return srv, ss
}

func createServerAndSessionWithInput(cfg *Config, state sessionState, input string) (*Server, *session, *fakeWriter) {
	r := newFakeReader(input)
	w := &fakeWriter{}
	srv, ss := createServerAndSessionWithRW(cfg, state, r, w)
	return srv, ss, w
}
