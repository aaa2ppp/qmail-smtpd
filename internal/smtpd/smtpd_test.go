package smtpd

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/pipeconn"
	"qmail-smtpd/internal/smtpd/safeio"
)

func TestSmtpd_Run(t *testing.T) {
	tests := []struct {
		name    string
		session session
		input   string
		want    []int
		wantErr bool
	}{
		{
			"all commands",
			session{openRelay: true},
			`nonexistent
STARTTLS
HELP
NOOP
VRFY
HELO
EHLO
MAIL FROM:<>
RCPT TO:<>
RCPT TO:<>
DATA
RSET
QUIT
`,
			[]int{220, 502, 502, 214, 250, 252, 250, 250, 250, 250, 250, 451, 250, 221},
			false,
		},
		{
			"rcpthost",
			session{rcptHosts: alwaysMatch{}},
			"HELO\nMAIL FROM:<>\nRCPT TO:<>\nQUIT\n",
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"!rcpthost",
			session{rcptHosts: alwaysNotMatch{}},
			"HELO\nMAIL FROM:<>\nRCPT TO:<>\nQUIT\n",
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"relayclient",
			session{rcptHosts: alwaysNotMatch{}, relayClient: true},
			"HELO\nMAIL FROM:<>\nRCPT TO:<>\nQUIT\n",
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom",
			session{badMailFrom: alwaysMatch{}, relayClient: true},
			"HELO\nMAIL FROM:<>\nRCPT TO:<>\nQUIT\n",
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"!badmailfrom",
			session{badMailFrom: alwaysNotMatch{}, relayClient: true},
			"HELO\nMAIL FROM:<>\nRCPT TO:<>\nQUIT\n",
			[]int{220, 250, 250, 250, 221},
			false,
		},
		{
			"badmailfrom & rcpthost",
			session{badMailFrom: alwaysMatch{}, rcptHosts: alwaysMatch{}},
			"HELO\nMAIL FROM:<>\nRCPT TO:<>\nQUIT\n",
			[]int{220, 250, 250, 553, 221},
			false,
		},
		{
			"mail first on rcpt",
			session{},
			"HELO\nRCPT TO:<>\nQUIT\n",
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"mail first on DATA",
			session{},
			"HELO\ndata\nQUIT\n",
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"rcpt first on DATA",
			session{},
			"HELO\nMAIL FROM:<>\ndata\nQUIT\n",
			[]int{220, 250, 250, 503, 221},
			false,
		},
		{
			"mail syntax error",
			session{},
			"HELO\nmail\nQUIT\n",
			[]int{220, 250, 555, 221},
			false,
		},
		{
			"rcpt syntax error",
			session{},
			"HELO\nMAIL FROM:<>\nRCPT\nQUIT\n",
			[]int{220, 250, 250, 555, 221},
			false,
		},
		{
			"no AUTH",
			session{},
			"EHLO\nAUTH\nQUIT\n",
			[]int{220, 250, 503, 221},
			false,
		},
		{
			"AUTH LOGIN - oops! need STARTTLS",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}},
			`EHLO
AUTH LOGIN dmFzeWFAcHVwa2luLm9yZwo=
QUIT
`,
			[]int{220, 250, 504, 221},
			false,
		},
		{
			"AUTH LOGIN - ok",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH LOGIN dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
QUIT
`,
			[]int{220, 250, 334, 235, 221},
			false,
		},
		{
			"AUTH LOGIN - oops! need base64 encoding",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH LOGIN vasya@pupkin.org
QUIT
`,
			[]int{220, 250, 501, 221},
			false,
		},
		{
			"AUTH LOGIN 2 - ok",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH LOGIN
dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
QUIT
`,
			[]int{220, 250, 334, 334, 235, 221},
			false,
		},
		{
			"AUTH LOGIN 2 - ok",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH LOGIN
dmFzeWFAcHVwa2luLm9yZwo=
bXkgc3Ryb25nIHBhc3N3b3JkCg==
QUIT
`,
			[]int{220, 250, 334, 334, 235, 221},
			false,
		},
		{
			"AUTH LOGIN 2 - oops! need base64 encoding",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH LOGIN
vasya@pupkin.org
QUIT
`,
			[]int{220, 250, 334, 501, 221},
			false,
		},
		{
			"AUTH PLAIN - oops! need STARTTLS",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}},
			`EHLO
AUTH PLAIN MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
QUIT
`,
			[]int{220, 250, 504, 221},
			false,
		},
		{
			"AUTH PLAIN - ok",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH PLAIN MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
QUIT
`,
			[]int{220, 250, 235, 221},
			false,
		},
		{
			"AUTH PLAIN - oops! need base64 encoding",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH PLAIN 12345` + "\x00" + `vasya@pupkin.org` + "\x00" + `my strong password
QUIT
`,
			[]int{220, 250, 501, 221},
			false,
		},
		{
			"AUTH PLAIN 2 - ok",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH PLAIN
MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=
QUIT
`,
			[]int{220, 250, 334, 235, 221},
			false,
		},
		{
			"AUTH PLAIN 2 - oops! need base64 encoding",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}, tlsEnabled: true},
			`EHLO
AUTH PLAIN
12345` + "\x00" + `vasya@pupkin.org` + "\x00" + `my strong password
QUIT
`,
			[]int{220, 250, 334, 501, 221},
			false,
		},
		{
			"AUTH CRAM-MD5 - oops! need base64 encoding",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}},
			`EHLO
AUTH CRAM-MD5
dmFzeWFAcHVwa2luLm9yZyBhNGZlYTY2YjJhYjA4ZjEyZGI5OTYyMTlmZTc3YTM1Yw==
QUIT
`,
			[]int{220, 250, 334, 235, 221},
			false,
		},
		{
			"AUTH CRAM-MD5 - oops! need base64 encoding",
			session{authFQDN: "localhost", auth: &fakeAuth{ok: true}},
			`EHLO
AUTH CRAM-MD5
vasya@pupkin.org a4fea66b2ab08f12db996219fe77a35c
QUIT
`,
			[]int{220, 250, 334, 501, 221},
			false,
		},
		{
			"DATA - ok",
			session{qmail: &fakeQueue{}, rcptHosts: alwaysMatch{}},
			addCr(`HELO localhost
MAIL FROM:<vasya@pupkin.org>
RCPT TO:<masha@pupkin.org>
DATA
Subject: Hello

Hello, Masha!
.
QUIT
`),
			[]int{220, 250, 250, 250, 354, 250, 221},
			false,
		},
		{
			"DATA - no CR",
			session{qmail: &fakeQueue{}, rcptHosts: alwaysMatch{}},
			delCr(`HELO localhost
MAIL FROM:<vasya@pupkin.org>
RCPT TO:<masha@pupkin.org>
DATA
Subject: Hello

Hello, Masha!
.
QUIT
`),
			[]int{220, 250, 250, 250, 354, 451},
			true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &fakeWriter{}
			conn := &pipeconn.Conn{
				Reader: newFakeReader(tt.input),
				Writer: out,
			}

			logOut := &bytes.Buffer{}

			io := safeio.New(conn, logOut, 0)
			ss := tt.session

			ss.ctx = context.Background()
			ss.io = io
			ss.env = env.Env{}
			ss.greeting = "example.org"

			commands := newSMTPCommands()
			var showLog bool

			if err := smtp_run(&ss, commands); (err != nil) != tt.wantErr {
				t.Errorf("\nSmtpd.Run() error = %v, wantErr %v", err, tt.wantErr)
				t.Logf("\n%s", logOut.String())
				showLog = true
			}

			got := extractCodes(out.String())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("\nSmtpd.Run() = \n%v, \nwant \n%v", got, tt.want)
				showLog = true
			}

			if showLog {
				t.Logf("\n%s", logOut.String())
			}
		})
	}
}
