package smtpd

import (
	"reflect"
	"strings"
	"testing"
)

func TestSmtpd_auth_prompt(t *testing.T) {
	type args struct {
		prompt string
	}
	tests := []struct {
		name  string
		cfg   *Config
		state sessionState
		args  args
		Want  string
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			args{""},
			"334 \r\n", // <SP> required
		},
		{
			"Hello, 世界",
			&Config{},
			sessionState{},
			args{"Hello, 世界"},
			"334 SGVsbG8sIOS4lueVjA==\r\n",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, ss, out := createServerAndSession(tt.cfg, tt.state)
			srv.auth_prompt(ss, tt.args.prompt)
			if out.String() != tt.Want {
				t.Errorf("out = %q, want %q", out.String(), tt.Want)
			}
		})
	}
}

func TestSmtpd_auth_gets(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		input     string
		want      string
		wantNoErr bool
		wantOut   string
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			"\r\n",
			"",
			true,
			"",
		},
		{
			"no base64",
			&Config{},
			sessionState{},
			"Hello, world!\r\n",
			"",
			false,
			"501 ",
		},
		{
			"base64",
			&Config{},
			sessionState{},
			"SGVsbG8sIOS4lueVjA==\r\n",
			"Hello, 世界",
			true,
			"",
		},
		{
			"*",
			&Config{},
			sessionState{},
			"*\n",
			"",
			false,
			"501 ",
		},
		// TODO: какое поведение должно быть в этом случае?
		// {
		// 	"leader spaces",
		// 	New(&Config{},
		// 	"  SGVsbG8sIOS4lueVjA==\r\n",
		// 	"",
		// 	false,
		// },
		// {
		// 	"finaller spaces",
		// 	New(&Config{},
		// 	"SGVsbG8sIOS4lueVjA==  \r\n",
		// 	"",
		// 	false,
		// },
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, ss, w := createServerAndSessionWithInput(tt.cfg, tt.state, tt.input)

			got, err := srv.auth_getln(ss)
			ss.Flush()

			if got != tt.want {
				t.Errorf("Smtpd.auth_gets() got = %v, want %v", got, tt.want)
			}
			if (err == nil) != tt.wantNoErr {
				t.Errorf("Smtpd.auth_gets() err = %v, want %v", err, !tt.wantNoErr)
			}
			if out := w.String(); !strings.HasPrefix(out, tt.wantOut) {
				t.Errorf("out = %q, want %q", out, tt.wantOut)
			}
		})
	}
}

func TestSmtpd_auth_login(t *testing.T) {
	type args struct {
		arg string
	}
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		args      args
		input     string
		want      authAttributes
		wantNoErr bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			args{""},
			"dmFzeWFAcHVwa2luLm9yZw==\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			authAttributes{
				user: "vasya@pupkin.org",
				pass: "my strong password",
			},
			true,
			[]int{334, 334},
		},
		{
			"no username1",
			&Config{},
			sessionState{},
			args{"="},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			authAttributes{},
			false,
			[]int{501},
		},
		{
			"no username2",
			&Config{},
			sessionState{},
			args{""},
			"\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		{
			"no password",
			&Config{},
			sessionState{},
			args{""},
			"dmFzeWFAcHVwa2luLm9yZw==\r\n\r\n",
			authAttributes{
				user: "vasya@pupkin.org",
			},
			false,
			[]int{334, 334, 501},
		},
		{
			"vasya@pupkin.org",
			&Config{},
			sessionState{},
			args{"dmFzeWFAcHVwa2luLm9yZw=="},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			authAttributes{
				user: "vasya@pupkin.org",
				pass: "my strong password",
			},
			true,
			[]int{334},
		},
		{
			"abort",
			&Config{},
			sessionState{},
			args{"dmFzeWFAcHVwa2luLm9yZw=="},
			"*\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, ss, w := createServerAndSessionWithInput(tt.cfg, tt.state, tt.input)

			got, err := srv.auth_login(ss, tt.args.arg)
			ss.Flush()

			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Smtpd.auth_login() got = %v, want %v", got, tt.want)
			}
			if (err == nil) != tt.wantNoErr {
				t.Errorf("Smtpd.auth_login() err = %v, want %v", err, !tt.wantNoErr)
			}
			out := w.String()
			if codes := extractCodes(out); !reflect.DeepEqual(codes, tt.wantCodes) {
				t.Logf("out = %v", out)
				t.Errorf("codes = %v, want %v", codes, tt.wantCodes)
			}
		})
	}
}

func TestSmtpd_auth_plain(t *testing.T) {
	type args struct {
		arg string
	}
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		args      args
		input     string
		want      authAttributes
		wantNoErr bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Config{},
			sessionState{},
			args{""},
			"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQA\r\n",
			authAttributes{
				user: "vasya@pupkin.org",
				pass: "my strong password",
			},
			true,
			[]int{334},
		},
		{
			"argument",
			&Config{},
			sessionState{},
			args{"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQA"},
			"",
			authAttributes{
				user: "vasya@pupkin.org",
				pass: "my strong password",
			},
			true,
			[]int{0}, // XXX if empty, extractCodes returns [0]
		},
		{
			"empty argument (=)",
			&Config{},
			sessionState{},
			args{"="},
			"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQA\r\n",
			authAttributes{},
			false,
			[]int{501},
		},
		{
			"no response",
			&Config{},
			sessionState{},
			args{""},
			"\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		{
			"abort",
			&Config{},
			sessionState{},
			args{""},
			"*\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, ss, w := createServerAndSessionWithInput(tt.cfg, tt.state, tt.input)

			got, err := srv.auth_plain(ss, tt.args.arg)
			ss.Flush()

			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Smtpd.auth_login() got = %v, want %v", got, tt.want)
			}
			if (err == nil) != tt.wantNoErr {
				t.Errorf("Smtpd.auth_login() err = %v, want %v", err, !tt.wantNoErr)
			}
			out := w.String()
			if codes := extractCodes(out); !reflect.DeepEqual(codes, tt.wantCodes) {
				t.Logf("out = %v", out)
				t.Errorf("codes = %v, want %v", codes, tt.wantCodes)
			}
		})
	}
}

func TestSmtpd_auth_cram(t *testing.T) {
	type args struct {
		arg string
	}
	tests := []struct {
		name      string
		cfg       *Config
		state     sessionState
		args      args
		input     string
		want      authAttributes
		wantNoErr bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Config{Hostname: "mx.pupkin.org"},
			sessionState{},
			args{""},
			"dmFzeWFAcHVwa2luLm9yZyBhNGZlYTY2YjJhYjA4ZjEyZGI5OTYyMTlmZTc3YTM1Yw==\r\n",
			authAttributes{
				user: "vasya@pupkin.org",
				pass: "<12345.1716902519@mx.pupkin.org>",
				resp: "a4fea66b2ab08f12db996219fe77a35c",
			},
			true,
			[]int{334},
		},
		{
			"empty argument (=)",
			&Config{Hostname: "mx.pupkin.org"},
			sessionState{},
			args{"="},
			"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQA\r\n",
			authAttributes{},
			false,
			[]int{501},
		},
		{
			"no response",
			&Config{},
			sessionState{},
			args{""},
			"\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		{
			"abort",
			&Config{},
			sessionState{},
			args{""},
			"*\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		// TODO: Add test cases.
	}

	attributesIsEqual := func(a1, a2 authAttributes) bool {
		if a1.user != a2.user || a1.resp != a2.resp {
			return false
		}
		// TODO: compare pass
		return true
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, ss, w := createServerAndSessionWithInput(tt.cfg, tt.state, tt.input)

			got, err := srv.auth_cram(ss, tt.args.arg)
			ss.Flush()

			if err == nil && !attributesIsEqual(got, tt.want) {
				t.Errorf("Smtpd.auth_login() got = %v, want %v", got, tt.want)
			}
			if (err == nil) != tt.wantNoErr {
				t.Errorf("Smtpd.auth_login() err = %v, want %v", err, !tt.wantNoErr)
			}
			out := w.String()
			if codes := extractCodes(out); !reflect.DeepEqual(codes, tt.wantCodes) {
				t.Logf("out = %v", out)
				t.Errorf("codes = %v, want %v", codes, tt.wantCodes)
			}
		})
	}
}
