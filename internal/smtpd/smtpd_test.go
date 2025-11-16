package smtpd

import (
	"context"
	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/pipeconn"
	"reflect"
	"testing"
	"time"
)

func TestSmtpd_Run(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		state   sessionState
		r       *fakeReader
		w       *fakeWriter
		want    []int
		wantErr bool
	}{
		{
			"all commands",
			&Config{},
			sessionState{},
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
			&Config{RcptHosts: alwaysMatch{}},
			sessionState{},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"!rcpthost",
			&Config{RcptHosts: alwaysNotMatch{}},
			sessionState{},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"relayclient",
			&Config{RcptHosts: alwaysNotMatch{}},
			sessionState{relayclientok: true},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom",
			&Config{BadMailFrom: alwaysMatch{}},
			sessionState{},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"!badmailfrom",
			&Config{BadMailFrom: alwaysNotMatch{}},
			sessionState{},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom & rcpthost",
			&Config{BadMailFrom: alwaysMatch{}, RcptHosts: alwaysMatch{}},
			sessionState{},
			newFakeReader("helo\nmail from:<>\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"read timeout",
			&Config{Timeout: 10 * time.Millisecond},
			sessionState{},
			newFakeReaderWithTimeout("quit\n", 100*time.Millisecond),
			&fakeWriter{},
			[]int{220, 451},
			true,
		},
		{
			"read error",
			&Config{Timeout: 10 * time.Millisecond},
			sessionState{},
			newFakeReaderAlwaysErr(),
			&fakeWriter{},
			[]int{220},
			true,
		},
		{
			"write timeout",
			&Config{Timeout: 10 * time.Millisecond},
			sessionState{},
			newFakeReader("quit\n"),
			&fakeWriter{timeout: 100 * time.Millisecond},
			[]int{},
			true,
		},
		{
			"mail from:<> first",
			&Config{},
			sessionState{},
			newFakeReader("helo\nrcpt to:<>\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"mail from:<> first 2",
			&Config{},
			sessionState{},
			newFakeReader("helo\ndata\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"rcpt to:<> first",
			&Config{},
			sessionState{},
			newFakeReader("helo\nmail from:<>\ndata\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 503, 221},
			false,
		},
		{
			"mail syntax error",
			&Config{},
			sessionState{},
			newFakeReader("helo\nmail\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 555, 221},
			false,
		},
		{
			"rcpt syntax error",
			&Config{},
			sessionState{},
			newFakeReader("helo\nmail from:<>\nrcpt\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 250, 555, 221},
			false,
		},
		{
			"no auth",
			&Config{},
			sessionState{},
			newFakeReader("ehlo\nauth\nquit\n"),
			&fakeWriter{},
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"auth login - oops! need starttls",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{},
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
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{},
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
			&Config{
				Qmail: &fakeQueue{},
			},
			sessionState{},
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
			&Config{
				Qmail: &fakeQueue{},
			},
			sessionState{},
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

	// xxx
	env := env.New([]string{
		"TCPLOCALHOST=mx.pupkin.org",
		"TCPREMOTEIP=192.168.69.69",
		"REMOTEHOST=vasya.pupkin.org",
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewServer(tt.cfg)
			conn := &pipeconn.Conn{
				Reader: tt.r,
				Writer: tt.w,
			}
			ss := srv.newSession(context.Background(), conn, env)
			ss.sessionState = tt.state

			// srv, ss := createServerAndSessionWithRW(tt.cfg, tt.state, tt.r, tt.w)
			// ss.env = env
			alreadyShowed := false

			if err := srv.run(ss); (err != nil) != tt.wantErr {
				if !alreadyShowed {
					t.Logf("\nSmtpd.Run() = %s", tt.w.String())
					alreadyShowed = true
				}
				t.Errorf("\nSmtpd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			got := extractCodes(tt.w.String())
			if tt.w.timeout == 0 && !reflect.DeepEqual(got, tt.want) {
				if !alreadyShowed {
					t.Logf("\nSmtpd.Run() = %s", tt.w.String())
					alreadyShowed = true
				}
				t.Errorf("\nSmtpd.Run() = %v, \nwant %v", got, tt.want)
			}
		})
	}
}
