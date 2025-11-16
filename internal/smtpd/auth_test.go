package smtpd

import (
	"reflect"
	"strings"
	"testing"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/pipeconn"
	"qmail-smtpd/internal/smtpd/safeio"
)

func Test_authHandler_challenge(t *testing.T) {
	const op = "authHandler.challenge"

	type args struct {
		challenge string
	}
	tests := []struct {
		name    string
		cfg     *Config
		state   sessionState
		args    args
		wantErr bool
		wantOut string
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			args{""},
			false,
			"334 \r\n", // <SP> required
		},
		{
			"Hello, 世界",
			&Config{},
			sessionState{},
			args{"Hello, 世界"},
			false,
			"334 SGVsbG8sIOS4lueVjA==\r\n",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &fakeReader{Reader: strings.NewReader("")}
			w := &fakeWriter{}
			conn := &pipeconn.Conn{
				Reader: r,
				Writer: w,
			}
			ss := &session{
				SafeIO:       safeio.New(conn, nil, tt.cfg.Timeout),
				sessionState: tt.state,
			}
			auth := authHandler{cfg: tt.cfg}

			err := auth.challenge(ss, tt.args.challenge)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: err = %v, want %v", op, err, tt.wantErr)
			}
			if w.String() != tt.wantOut {
				t.Errorf("%s: out = %q, want %q", op, w.String(), tt.wantOut)
			}
		})
	}
}

func Test_authHandler_response(t *testing.T) {
	const op = "authHandler.response"

	tests := []struct {
		name    string
		cfg     *Config
		state   sessionState
		input   string
		want    string
		wantErr bool
		wantOut string
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			"\r\n",
			"",
			false,
			"",
		},
		{
			"no base64",
			&Config{},
			sessionState{},
			"Hello, world!\r\n",
			"",
			true,
			"501 ",
		},
		{
			"base64",
			&Config{},
			sessionState{},
			"SGVsbG8sIOS4lueVjA==\r\n",
			"Hello, 世界",
			false,
			"",
		},
		{
			"*",
			&Config{},
			sessionState{},
			"*\n",
			"",
			true,
			"501 ",
		},
		{
			"leader spaces",
			&Config{},
			sessionState{},
			"  SGVsbG8sIOS4lueVjA==\r\n",
			"",
			true,
			"501 ",
		},
		{
			"finaller spaces",
			&Config{},
			sessionState{},
			"SGVsbG8sIOS4lueVjA==  \r\n",
			"",
			true,
			"501 ",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &fakeReader{Reader: strings.NewReader(tt.input)}
			w := &fakeWriter{}
			conn := &pipeconn.Conn{
				Reader: r,
				Writer: w,
			}
			ss := &session{
				SafeIO:       safeio.New(conn, nil, tt.cfg.Timeout),
				sessionState: tt.state,
			}
			auth := authHandler{cfg: tt.cfg}

			got, err := auth.response(ss)
			ss.Flush()

			if (err != nil) != tt.wantErr {
				t.Errorf("%s: err = %v, want %v", op, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("%s: got = %v, want %v", op, got, tt.want)
			}
			if out := w.String(); !strings.HasPrefix(out, tt.wantOut) {
				t.Errorf("%s: out = %q, want %q", op, out, tt.wantOut)
			}
		})
	}
}

func Test_authHandler_login(t *testing.T) {
	const op = "authHandler.login"

	type credentials = plainCredentials

	type args struct {
		arg string
	}
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		args      args
		input     string
		want      *credentials
		wantErr   bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			args{""},
			"dmFzeWFAcHVwa2luLm9yZw==\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			&credentials{
				AuthZID: "vasya@pupkin.org",
				AuthCID: "vasya@pupkin.org",
				Passwd:  "my strong password",
			},
			false,
			[]int{334, 334},
		},
		{
			"no username1",
			&Config{},
			sessionState{},
			args{"="},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			nil,
			true,
			[]int{501},
		},
		{
			"no username2",
			&Config{},
			sessionState{},
			args{""},
			"\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			nil,
			true,
			[]int{334, 501},
		},
		{
			"no password",
			&Config{},
			sessionState{},
			args{""},
			"dmFzeWFAcHVwa2luLm9yZw==\r\n\r\n",
			nil,
			true,
			[]int{334, 334, 501},
		},
		{
			"vasya@pupkin.org",
			&Config{},
			sessionState{},
			args{"dmFzeWFAcHVwa2luLm9yZw=="},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			&credentials{
				AuthZID: "vasya@pupkin.org",
				AuthCID: "vasya@pupkin.org",
				Passwd:  "my strong password",
			},
			false,
			[]int{334},
		},
		{
			"abort",
			&Config{},
			sessionState{},
			args{"dmFzeWFAcHVwa2luLm9yZw=="},
			"*\r\n",
			nil,
			true,
			[]int{334, 501},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &fakeReader{Reader: strings.NewReader(tt.input)}
			w := &fakeWriter{}
			conn := &pipeconn.Conn{
				Reader: r,
				Writer: w,
			}
			ss := &session{
				SafeIO:       safeio.New(conn, nil, tt.cfg.Timeout),
				sessionState: tt.state,
			}
			auth := authHandler{cfg: tt.cfg}

			got, err := auth.login(ss, tt.args.arg)
			ss.Flush()

			if (err != nil) != tt.wantErr {
				t.Errorf("%s: err = %v, want %v", op, err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s: got = %+v, want %+v", op, got, tt.want)
			}
			out := w.String()
			if codes := extractCodes(out); !reflect.DeepEqual(codes, tt.wantCodes) {
				t.Logf("%s: out = %v", op, out)
				t.Errorf("%s: codes = %v, want %v", op, codes, tt.wantCodes)
			}
		})
	}
}

func Test_authHandler_plain(t *testing.T) {
	const op = "authHandler.plain"

	type credentials = plainCredentials

	type args struct {
		arg string
	}
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		args      args
		input     string
		want      *credentials
		wantErr   bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			args{""},
			"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQ=\r\n",
			&credentials{
				AuthZID: "12345",
				AuthCID: "vasya@pupkin.org",
				Passwd:  "my strong password",
			},
			false,
			[]int{334},
		},
		{
			"argument",
			&Config{},
			sessionState{},
			args{"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQ="},
			"",
			&credentials{
				AuthZID: "12345",
				AuthCID: "vasya@pupkin.org",
				Passwd:  "my strong password",
			},
			false,
			[]int{},
		},
		{
			"empty argument (=)",
			&Config{},
			sessionState{},
			args{"="},
			"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQ=\r\n",
			nil,
			true,
			[]int{501},
		},
		{
			"no response",
			&Config{},
			sessionState{},
			args{""},
			"\r\n",
			nil,
			true,
			[]int{334, 501},
		},
		{
			"abort",
			&Config{},
			sessionState{},
			args{""},
			"*\r\n",
			nil,
			true,
			[]int{334, 501},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &fakeReader{Reader: strings.NewReader(tt.input)}
			w := &fakeWriter{}
			conn := &pipeconn.Conn{
				Reader: r,
				Writer: w,
			}
			ss := &session{
				SafeIO:       safeio.New(conn, nil, tt.cfg.Timeout),
				sessionState: tt.state,
			}
			auth := authHandler{cfg: tt.cfg}

			got, err := auth.plain(ss, tt.args.arg)
			ss.Flush()

			if (err != nil) != tt.wantErr {
				t.Errorf("%s: err = %v, want %v", op, err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s: got = %v, want %v", op, got, tt.want)
			}
			out := w.String()
			if codes := extractCodes(out); !reflect.DeepEqual(codes, tt.wantCodes) {
				t.Logf("%s: out = %v", op, out)
				t.Errorf("%s: codes = %v, want %v", op, codes, tt.wantCodes)
			}
		})
	}
}

func Test_authHandler_cram(t *testing.T) {
	const op = "authHandler.cram"

	type credentials = cramCredentials

	type args struct {
		arg string
	}
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		args      args
		input     string
		want      *credentials
		wantErr   bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Config{AuthFQDN: "mx.pupkin.org"},
			sessionState{},
			args{""},
			"dmFzeWFAcHVwa2luLm9yZyAwMTIzNDU2Nzg5QUJDREVGMDEyMzQ1Njc4OWFiY2RlZg==\r\n",
			&credentials{
				Username:  "vasya@pupkin.org",
				Hexdigest: "0123456789ABCDEF0123456789abcdef",
			},
			false,
			[]int{334},
		},
		{
			"empty argument (=)",
			&Config{AuthFQDN: "mx.pupkin.org"},
			sessionState{},
			args{"="},
			"dmFzeWFAcHVwa2luLm9yZyAwMTIzNDU2Nzg5QUJDREVGMDEyMzQ1Njc4OWFiY2RlZg==\r\n",
			nil,
			true,
			[]int{501},
		},
		{
			"no response",
			&Config{},
			sessionState{},
			args{""},
			"\r\n",
			nil,
			true,
			[]int{334, 501},
		},
		{
			"abort",
			&Config{},
			sessionState{},
			args{""},
			"*\r\n",
			nil,
			true,
			[]int{334, 501},
		},
		// TODO: Add test cases.
	}

	checkCredentials := func(got Credentials, want *credentials) bool {
		cred, ok := got.(*credentials)
		if !ok {
			return false
		}
		// we don't check the password (it's our challenge)
		if cred.Username != want.Username {
			return false
		}
		if cred.Hexdigest != want.Hexdigest {
			return false
		}
		return true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &fakeReader{Reader: strings.NewReader(tt.input)}
			w := &fakeWriter{}
			conn := &pipeconn.Conn{
				Reader: r,
				Writer: w,
			}
			ss := &session{
				SafeIO:       safeio.New(conn, nil, tt.cfg.Timeout),
				sessionState: tt.state,
			}
			auth := authHandler{cfg: tt.cfg}

			got, err := auth.cram(ss, tt.args.arg)
			ss.Flush()

			if (err != nil) != tt.wantErr {
				t.Errorf("%s: err = %v, want %v", op, err, tt.wantErr)
			}
			if err == nil && !checkCredentials(got.(*credentials), tt.want) {
				t.Errorf("%s: got = %v, want %v", op, got, tt.want)
			}
			out := w.String()
			if codes := extractCodes(out); !reflect.DeepEqual(codes, tt.wantCodes) {
				t.Logf("%s: out = %v", op, out)
				t.Errorf("%s: codes = %v, want %v", op, codes, tt.wantCodes)
			}
		})
	}
}

func TestServer_smtp_auth(t *testing.T) {
	const op = "Server.smtp_auth"

	type args struct {
		arg string
	}
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		args      args
		input     string
		wantErr   bool
		wantCodes []int
	}{
		{
			"login one line",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"login dmFzeWFAcHVwa2luLm9yZwo="},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{334, 235},
		},
		{
			"login multi line",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"login"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{334, 334, 235},
		},
		{
			"login no tls",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{},
			args{"login dmFzeWFAcHVwa2luLm9yZwo="},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{504},
		},
		{
			"plain single line",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"plain MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo="},
			"",
			false,
			[]int{235},
		},
		{
			"plain multi line",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"plain"},
			"MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo=\r\n",
			false,
			[]int{334, 235},
		},
		{
			"plain no tls",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{},
			args{"plain MTIzNDUAdmFzeWFAcHVwa2luAG15IHN0cm9uZyBwYXNzd29yZAo="},
			"",
			false,
			[]int{504},
		},
		{
			"cram-md5 tls",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"cram-md5"},
			"dmFzeWFAcHVwa2luLm9yZyAwMTIzNDU2Nzg5QUJDREVGMDEyMzQ1Njc4OWFiY2RlZg==\r\n",
			false,
			[]int{334, 235},
		},
		{
			"cram-md5 no tls",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{},
			args{"cram-md5"},
			"dmFzeWFAcHVwa2luLm9yZyAwMTIzNDU2Nzg5QUJDREVGMDEyMzQ1Njc4OWFiY2RlZg==\r\n",
			false,
			[]int{334, 235},
		},
		{
			"argument not base64",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"login vasya@pupkin.org"},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{501},
		},
		{
			"first response not base64",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"login"},
			"vasya@pupkin.org\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{334, 501},
		},
		{
			"second response not base64",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"login"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\nmy strong password\r\n",
			false,
			[]int{334, 334, 501},
		},
		{
			"canceled on first response",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"login"},
			"*\r\n",
			false,
			[]int{334, 501},
		},
		{
			"canceled on second response",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: true}},
			sessionState{tlsEnabled: true},
			args{"login"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\n*\r\n",
			false,
			[]int{334, 334, 501},
		},
		{
			"authorization failed",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: false}},
			sessionState{tlsEnabled: true},
			args{"login"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{334, 334, 535},
		},
		{
			"no auth",
			&Config{AuthFQDN: "localhost", Auth: nil},
			sessionState{tlsEnabled: true},
			args{"login"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{503},
		},
		{
			"already authenticated",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: false}},
			sessionState{tlsEnabled: true, authorized: true},
			args{"login"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{503},
		},
		{
			"mail transaction",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: false}},
			sessionState{tlsEnabled: true, seenmail: true},
			args{"login"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{503},
		},
		{
			"unknown mechanism",
			&Config{AuthFQDN: "localhost", Auth: &fakeAuth{ok: false}},
			sessionState{tlsEnabled: true},
			args{"unknown"},
			"dmFzeWFAcHVwa2luLm9yZwo=\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			false,
			[]int{504},
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &fakeReader{Reader: strings.NewReader(tt.input)}
			w := &fakeWriter{}
			conn := &pipeconn.Conn{
				Reader: r,
				Writer: w,
			}
			ss := &session{
				env:          env.New(nil),
				SafeIO:       safeio.New(conn, nil, tt.cfg.Timeout),
				sessionState: tt.state,
			}
			d := NewServer(tt.cfg)

			oldAuthorized := ss.authorized

			_ = d.smtp_auth(ss, tt.args.arg)
			err := ss.Flush()
			out := w.String()

			var outShowed bool
			if (err != nil) != tt.wantErr {
				if !outShowed {
					t.Logf("%s: out = %v", op, out)
					outShowed = true
				}
				t.Errorf("%s: err = %v, want %v", op, err, tt.wantErr)
			}

			codes := extractCodes(out)
			if !reflect.DeepEqual(codes, tt.wantCodes) {
				if !outShowed {
					t.Logf("%s: out = %v", op, out)
					outShowed = true
				}
				t.Errorf("%s: codes = %v, want %v", op, codes, tt.wantCodes)
			}

			successAuth := len(codes) > 0 && codes[len(codes)-1] == 235
			wantAuthorized := successAuth || oldAuthorized
			if ss.authorized != wantAuthorized {
				if !outShowed {
					t.Logf("%s: out = %v", op, out)
					outShowed = true
				}
				t.Errorf("%s: ss.authorized = %v, want %v", op, ss.authorized, wantAuthorized)
			}
		})
	}
}

func Test_generateCRAMChallenge(t *testing.T) {
	const (
		hostname   = "localhost.localdomain"
		checkCount = 1000
	)

	resultSet := make(map[string]struct{}, checkCount)
	for range checkCount {
		got := generateCRAMChallenge(hostname)
		if !checkMsgID(got) {
			t.Fatalf("generateCRAMChallenge() = %v, want valid msg-id", got)
		}
		if !strings.HasSuffix(got, "@"+hostname+">") {
			t.Fatalf("generateCRAMChallenge() = %v, want suffix '@%s>'", got, hostname)
		}
		if _, ok := resultSet[got]; ok {
			t.Fatalf("generateCRAMChallenge() = %v (duplicated), must be unique", got)
		}
		resultSet[got] = struct{}{}
	}
}
