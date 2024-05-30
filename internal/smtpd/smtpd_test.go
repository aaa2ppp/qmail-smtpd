package smtpd

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"qmail-smtpd/internal/conn"
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

func newFakeReader(s string) *fakeReader { return &fakeReader{Reader: strings.NewReader(s)} }
func newFakeReaderTimeout(s string, timeout time.Duration) *fakeReader {
	r := newFakeReader(s)
	r.timeout = timeout
	return r
}
func newReaderAlwaysErr() *fakeReader                   { return &fakeReader{alwaysErr: true} }
func (r *fakeReader) SetReadDeadline(t time.Time) error { r.deadline = t; return nil }
func (r *fakeReader) Close() error                      { return nil }
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
	buf      strings.Builder
	timeout  time.Duration
	deadline time.Time
}

func (w *fakeWriter) SetWriteDeadline(t time.Time) error { w.deadline = t; return nil }
func (r *fakeWriter) Close() error                       { return nil }
func (w *fakeWriter) Write(b []byte) (int, error) {
	if !w.deadline.Equal(time.Time{}) && w.deadline.Before(time.Now().Add(w.timeout)) {
		time.Sleep(w.timeout)
		return 0, os.ErrDeadlineExceeded
	}
	return w.buf.Write(b)
}

type qmailQueue struct {
	result string
}

func (qq *qmailQueue) Open() (QmailQueue, error) {
	return qq, nil
}

func (qq *qmailQueue) Pid() int      { return 7777 }
func (qq *qmailQueue) Putc(_ byte)   {}
func (qq *qmailQueue) Puts(_ string) {}
func (qq *qmailQueue) From(_ string) {}
func (qq *qmailQueue) To(_ string)   {}
func (qq *qmailQueue) Fail()         { qq.result = "D*** Fail() called ***" }
func (qq *qmailQueue) Close() string { return qq.result }

var (
	_ Qmail      = (*qmailQueue)(nil)
	_ QmailQueue = (*qmailQueue)(nil)
)

func TestSmtpd_Run(t *testing.T) {
	tests := []struct {
		name    string
		d       *Smtpd
		r       conn.Reader
		w       *fakeWriter
		want    []int
		wantErr bool
	}{
		{
			"all commands",
			&Smtpd{},
			newFakeReader(`nonexistent
starttls
help
noop
vrfy
helo
ehlo
mail from:<>
rcpt to:<>
rcpt to:<>
data
rset
quit
`),
			&fakeWriter{},
			[]int{220, 502, 502, 214, 250, 252, 250, 250, 250, 250, 250, 451, 250, 221},
			false,
		},
		{
			"rcpthost",
			&Smtpd{RcptHosts: alwaysMatch{}},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"!rcpthost",
			&Smtpd{RcptHosts: alwaysNotMatch{}},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"relayclient",
			&Smtpd{RcptHosts: alwaysNotMatch{}, RelayClientOk: true},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom",
			&Smtpd{BadMailFrom: alwaysMatch{}},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"!badmailfrom",
			&Smtpd{BadMailFrom: alwaysNotMatch{}},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom & rcpthost",
			&Smtpd{BadMailFrom: alwaysMatch{}, RcptHosts: alwaysMatch{}},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"read timeout",
			&Smtpd{Timeout: 1 * time.Millisecond},
			newFakeReaderTimeout("quit\n", 10*time.Millisecond),
			&fakeWriter{},
			[]int{220, 451},
			true,
		},
		{
			"read error",
			&Smtpd{Timeout: 1},
			newReaderAlwaysErr(),
			&fakeWriter{},
			[]int{220},
			true,
		},
		{
			"write timeout",
			&Smtpd{Timeout: 1 * time.Millisecond},
			newFakeReader("quit\n"),
			&fakeWriter{timeout: 10 * time.Millisecond},
			[]int{},
			true,
		},
		{
			"mail from:<> first",
			&Smtpd{},
			newFakeReader("helo\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"mail from:<> first 2",
			&Smtpd{},
			newFakeReader("helo\ndata\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"rcpt to:<> first",
			&Smtpd{},
			newFakeReader("helo\nmail from:<>\ndata\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 503, 221},
			false,
		},
		{
			"mail syntax error",
			&Smtpd{},
			newFakeReader("helo\nmail\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 555, 221},
			false,
		},
		{
			"rcpt syntax error",
			&Smtpd{},
			newFakeReader("helo\nmail from:<>\nrcpt\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 555, 221},
			false,
		},
		{
			"no auth",
			&Smtpd{},
			newFakeReader("ehlo\nauth\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"auth login - oops! need starttls",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			newFakeReader(`ehlo
auth login dmFzeWFAcHVwa2luLm9yZwo=
quit
`),
			&fakeWriter{},
			[]int{220, 250, 504, 221},
			false,
		},
		{
			"auth login - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth login dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 235, 221},
			false,
		},
		{
			"auth login - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth login vasya@pupkin.org
quit
`),
			&fakeWriter{},
			[]int{220, 250, 501, 221},
			false,
		},
		{
			"auth login2 - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth login
dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 334, 235, 221},
			false,
		},
		{
			"auth login2 - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth login
dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 334, 235, 221},
			false,
		},
		{
			"auth login2 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth login
vasya@pupkin.org
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 501, 221},
			false,
		},
		{
			"auth plain - oops! need starttls",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			newFakeReader(`ehlo
auth plain MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
quit
`),
			&fakeWriter{},
			[]int{220, 250, 504, 221},
			false,
		},
		{
			"auth plain - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth plain MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
quit
`),
			&fakeWriter{},
			[]int{220, 250, 235, 221},
			false,
		},
		{
			"auth plain - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth plain 12345` + "\x00" + `vasya@pupkin.org` + "\x00" + `my strong password
quit
`),
			&fakeWriter{},
			[]int{220, 250, 501, 221},
			false,
		},
		{
			"auth plain2 - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth plain 
MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 235, 221},
			false,
		},
		{
			"auth plain2 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}, tlsEnabled: true},
			newFakeReader(`ehlo
auth plain
12345` + "\x00" + `vasya@pupkin.org` + "\x00" + `my strong password
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 501, 221},
			false,
		},
		{
			"auth cram-md5 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			newFakeReader(`ehlo
auth cram-md5
dmFzeWFAcHVwa2luLm9yZyBhNGZlYTY2YjJhYjA4ZjEyZGI5OTYyMTlmZTc3YTM1Yw==
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 235, 221},
			false,
		},
		{
			"auth cram-md5 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			newFakeReader(`ehlo
auth cram-md5
vasya@pupkin.org a4fea66b2ab08f12db996219fe77a35c
quit
`),
			&fakeWriter{},
			[]int{220, 250, 334, 501, 221},
			false,
		},
		{
			"data - ok",
			&Smtpd{
				LocalHost:  "mx.pupkin.org",
				RemoteIP:   "192.168.69.69",
				RemoteHost: "vasya.pupkin.org",
				RemoteInfo: "42",
				Qmail:      &qmailQueue{},
			},
			newFakeReader(addCr(`helo localhost
mail from:<vasya@pupkin.org>
rcpt to:<masha@pupkin.org>
data
Subject: Hello

Hello, Masha!
.
quit
`)),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 354, 250, 221},
			false,
		},
		{
			"data - no CR",
			&Smtpd{
				LocalHost:  "mx.pupkin.org",
				RemoteIP:   "192.168.69.69",
				RemoteHost: "vasya.pupkin.org",
				RemoteInfo: "42",
				Qmail:      &qmailQueue{},
			},
			newFakeReader(delCr(`helo localhost
mail from:<vasya@pupkin.org>
rcpt to:<masha@pupkin.org>
data
Subject: Hello

Hello, Masha!
.
quit
`)),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 354, 451},
			true,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var showed bool
			conn := &conn.Conn{Reader: tt.r, Writer: tt.w}
			if err := tt.d.Run(conn); (err != nil) != tt.wantErr {
				if !showed {
					t.Logf("\nSmtpd.Run() = %s", tt.w.buf.String())
					showed = true
				}
				t.Errorf("\nSmtpd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := extractCodes(tt.w.buf.String())
			if tt.w.timeout == 0 && !reflect.DeepEqual(got, tt.want) {
				if !showed {
					t.Logf("\nSmtpd.Run() = %s", tt.w.buf.String())
					showed = true
				}
				t.Errorf("\nSmtpd.Run() = %v, \nwant %v", got, tt.want)
			}
		})
	}
}

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
