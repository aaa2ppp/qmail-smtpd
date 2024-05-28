package smtpd

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

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

type reader struct {
	*strings.Reader
	timeout time.Duration
}

func newReader(s string, timeout time.Duration) *reader {
	return &reader{
		strings.NewReader(s),
		timeout,
	}
}

func (r *reader) Read(b []byte) (int, error) {
	if r.timeout > 0 {
		time.Sleep(r.timeout)
	}
	return r.Reader.Read(b)
}

type badReader struct{}

func (r *badReader) Read(_ []byte) (int, error) {
	return 0, errors.New("io error")
}

type writer struct {
	strings.Builder
	timeout time.Duration
}

func (w *writer) Write(b []byte) (int, error) {
	if w.timeout > 0 {
		time.Sleep(w.timeout)
	}
	return w.Builder.Write(b)
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
		r       io.Reader
		w       *writer
		want    []int
		wantErr bool
	}{
		{
			"all commands",
			&Smtpd{},
			strings.NewReader(`nonexistent
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
			&writer{},
			[]int{200, 502, 214, 250, 252, 250, 250, 250, 250, 250, 451, 250, 221},
			false,
		},
		{
			"rcpthost",
			&Smtpd{RcptHosts: alwaysMatch{}},
			strings.NewReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 250, 221},
			false,
		},
		{
			"!rcpthost",
			&Smtpd{RcptHosts: alwaysNotMatch{}},
			strings.NewReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 553, 221},
			false,
		},
		{
			"relayclient",
			&Smtpd{RcptHosts: alwaysNotMatch{}, RelayClientOk: true},
			strings.NewReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom",
			&Smtpd{BadMailFrom: alwaysMatch{}},
			strings.NewReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 553, 221},
			false,
		},
		{
			"!badmailfrom",
			&Smtpd{BadMailFrom: alwaysNotMatch{}},
			strings.NewReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom & rcpthost",
			&Smtpd{BadMailFrom: alwaysMatch{}, RcptHosts: alwaysMatch{}},
			strings.NewReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 553, 221},
			false,
		},
		{
			"read timeout",
			&Smtpd{Timeout: 1 * time.Millisecond},
			newReader("quit\n", 10*time.Millisecond),
			&writer{},
			[]int{200, 451},
			true,
		},
		{
			"read error",
			&Smtpd{Timeout: 1},
			&badReader{},
			&writer{},
			[]int{200},
			true,
		},
		{
			"write timeout",
			&Smtpd{Timeout: 1 * time.Millisecond},
			strings.NewReader("quit\n"),
			&writer{timeout: 10 * time.Millisecond},
			[]int{},
			true,
		},
		{
			"mail from:<> first",
			&Smtpd{},
			strings.NewReader("helo\nrcpt to:<>\nquit\n"),
			&writer{},
			[]int{200, 250, 503, 221},
			false,
		},
		{
			"mail from:<> first 2",
			&Smtpd{},
			strings.NewReader("helo\ndata\nquit\n"),
			&writer{},
			[]int{200, 250, 503, 221},
			false,
		},
		{
			"rcpt to:<> first",
			&Smtpd{},
			strings.NewReader("helo\nmail from:<>\ndata\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 503, 221},
			false,
		},
		{
			"mail syntax error",
			&Smtpd{},
			strings.NewReader("helo\nmail\nquit\n"),
			&writer{},
			[]int{200, 250, 555, 221},
			false,
		},
		{
			"rcpt syntax error",
			&Smtpd{},
			strings.NewReader("helo\nmail from:<>\nrcpt\nquit\n"),
			&writer{},
			[]int{200, 250, 250, 555, 221},
			false,
		},
		{
			"no auth",
			&Smtpd{},
			strings.NewReader("ehlo\nauth\nquit\n"),
			&writer{},
			[]int{200, 250, 503, 221},
			false,
		},
		{
			"auth login - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth login dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
quit
`),
			&writer{},
			[]int{200, 250, 334, 235, 221},
			false,
		},
		{
			"auth login - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth login vasya@pupkin.org
quit
`),
			&writer{},
			[]int{200, 250, 501, 221},
			false,
		},
		{
			"auth login2 - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth login
dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
quit
`),
			&writer{},
			[]int{200, 250, 334, 334, 235, 221},
			false,
		},
		{
			"auth login2 - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth login
dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
quit
`),
			&writer{},
			[]int{200, 250, 334, 334, 235, 221},
			false,
		},
		{
			"auth login2 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth login
vasya@pupkin.org
quit
`),
			&writer{},
			[]int{200, 250, 334, 501, 221},
			false,
		},
		{
			"auth plain - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth plain MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
quit
`),
			&writer{},
			[]int{200, 250, 235, 221},
			false,
		},
		{
			"auth plain - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth plain 12345` + "\x00" + `vasya@pupkin.org` + "\x00" + `my strong password
quit
`),
			&writer{},
			[]int{200, 250, 501, 221},
			false,
		},
		{
			"auth plain2 - ok",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth plain 
MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
quit
`),
			&writer{},
			[]int{200, 250, 334, 235, 221},
			false,
		},
		{
			"auth plain2 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth plain
12345` + "\x00" + `vasya@pupkin.org` + "\x00" + `my strong password
quit
`),
			&writer{},
			[]int{200, 250, 334, 501, 221},
			false,
		},
		{
			"auth cram-md5 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth cram-md5
dmFzeWFAcHVwa2luLm9yZyBhNGZlYTY2YjJhYjA4ZjEyZGI5OTYyMTlmZTc3YTM1Yw==
quit
`),
			&writer{},
			[]int{200, 250, 334, 235, 221},
			false,
		},
		{
			"auth cram-md5 - oops! need base64 encoding",
			&Smtpd{Hostname: "localhost", Auth: alwaysAuth{}},
			strings.NewReader(`ehlo
auth cram-md5
vasya@pupkin.org a4fea66b2ab08f12db996219fe77a35c
quit
`),
			&writer{},
			[]int{200, 250, 334, 501, 221},
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
			strings.NewReader(addCr(`helo localhost
mail from:<vasya@pupkin.org>
rcpt to:<masha@pupkin.org>
data
Subject: Hello

Hello, Masha!
.
quit
`)),
			&writer{},
			[]int{200, 250, 250, 250, 354, 250, 221},
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
			strings.NewReader(delCr(`helo localhost
mail from:<vasya@pupkin.org>
rcpt to:<masha@pupkin.org>
data
Subject: Hello

Hello, Masha!
.
quit
`)),
			&writer{},
			[]int{200, 250, 250, 250, 354, 451},
			true,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var showed bool
			if err := tt.d.Run(tt.r, tt.w); (err != nil) != tt.wantErr {
				if !showed {
					t.Logf("\nSmtpd.Run() = %s", tt.w.String())
					showed = true
				}
				t.Errorf("\nSmtpd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := extractCodes(tt.w.String())
			if tt.w.timeout == 0 && !reflect.DeepEqual(got, tt.want) {
				if !showed {
					t.Logf("\nSmtpd.Run() = %s", tt.w.String())
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
