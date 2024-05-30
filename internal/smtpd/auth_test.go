package smtpd

import (
	"bufio"
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSmtpd_auth_prompt(t *testing.T) {
	type args struct {
		prompt string
	}
	tests := []struct {
		name string
		d    *Smtpd
		args args
		Want string
	}{
		{
			"<empty>",
			&Smtpd{},
			args{""},
			"334 \r\n", // <SP> required
		},
		{
			"Hello, 世界",
			&Smtpd{},
			args{"Hello, 世界"},
			"334 SGVsbG8sIOS4lueVjA==\r\n",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			tt.d.ssout = bufio.NewWriter(w)
			tt.d.auth_prompt(tt.args.prompt)
			if w.String() != tt.Want {
				t.Errorf("out = %q, want %q", w.String(), tt.Want)
			}
		})
	}
}

func TestSmtpd_auth_gets(t *testing.T) {
	tests := []struct {
		name    string
		d       *Smtpd
		input   string
		want    string
		want1   bool
		wantOut string
	}{
		{
			"<empty>",
			&Smtpd{},
			"\r\n",
			"",
			true,
			"",
		},
		{
			"no base64",
			&Smtpd{},
			"Hello, world!\r\n",
			"",
			false,
			"501 ",
		},
		{
			"base64",
			&Smtpd{},
			"SGVsbG8sIOS4lueVjA==\r\n",
			"Hello, 世界",
			true,
			"",
		},
		{
			"*",
			&Smtpd{},
			"*\n",
			"",
			false,
			"501 ",
		},
		// TODO: какое поведение должно быть в этом случае?
		// {
		// 	"leader spaces",
		// 	&Smtpd{},
		// 	"  SGVsbG8sIOS4lueVjA==\r\n",
		// 	"",
		// 	false,
		// },
		// {
		// 	"finaller spaces",
		// 	&Smtpd{},
		// 	"SGVsbG8sIOS4lueVjA==  \r\n",
		// 	"",
		// 	false,
		// },
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			tt.d.ssout = bufio.NewWriter(w)
			tt.d.ssin = bufio.NewReader(strings.NewReader(tt.input))
			got, got1 := tt.d.auth_getln()
			tt.d.flush()
			if got != tt.want {
				t.Errorf("Smtpd.auth_gets() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Smtpd.auth_gets() got1 = %v, want %v", got1, tt.want1)
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
		d         *Smtpd
		args      args
		input     string
		want      authAttributes
		want1     bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Smtpd{},
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
			&Smtpd{},
			args{"="},
			"bXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			authAttributes{},
			false,
			[]int{501},
		},
		{
			"no username2",
			&Smtpd{},
			args{""},
			"\r\nbXkgc3Ryb25nIHBhc3N3b3Jk\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		{
			"no password",
			&Smtpd{},
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
			&Smtpd{},
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
			&Smtpd{},
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
			w := &bytes.Buffer{}
			tt.d.ssout = bufio.NewWriter(w)
			tt.d.ssin = bufio.NewReader(strings.NewReader(tt.input))

			got, got1 := tt.d.auth_login(tt.args.arg)
			tt.d.flush()

			if got1 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Smtpd.auth_login() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Smtpd.auth_login() got1 = %v, want %v", got1, tt.want1)
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
		d         *Smtpd
		args      args
		input     string
		want      authAttributes
		want1     bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Smtpd{},
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
			&Smtpd{},
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
			&Smtpd{},
			args{"="},
			"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQA\r\n",
			authAttributes{},
			false,
			[]int{501},
		},
		{
			"no response",
			&Smtpd{},
			args{""},
			"\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		{
			"abort",
			&Smtpd{},
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
			w := &bytes.Buffer{}
			tt.d.ssout = bufio.NewWriter(w)
			tt.d.ssin = bufio.NewReader(strings.NewReader(tt.input))

			got, got1 := tt.d.auth_plain(tt.args.arg)
			tt.d.flush()

			if got1 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Smtpd.auth_login() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Smtpd.auth_login() got1 = %v, want %v", got1, tt.want1)
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
		d         *Smtpd
		args      args
		input     string
		want      authAttributes
		want1     bool
		wantCodes []int
	}{
		{
			"<empty>",
			&Smtpd{Hostname: "mx.pupkin.org"},
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
			&Smtpd{Hostname: "mx.pupkin.org"},
			args{"="},
			"MTIzNDUAdmFzeWFAcHVwa2luLm9yZwBteSBzdHJvbmcgcGFzc3dvcmQA\r\n",
			authAttributes{},
			false,
			[]int{501},
		},
		{
			"no response",
			&Smtpd{},
			args{""},
			"\r\n",
			authAttributes{},
			false,
			[]int{334, 501},
		},
		{
			"abort",
			&Smtpd{},
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
			w := &bytes.Buffer{}
			tt.d.ssout = bufio.NewWriter(w)
			tt.d.ssin = bufio.NewReader(strings.NewReader(tt.input))

			got, got1 := tt.d.auth_cram(tt.args.arg)
			tt.d.flush()

			if got1 && !attributesIsEqual(got, tt.want) {
				t.Errorf("Smtpd.auth_login() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Smtpd.auth_login() got1 = %v, want %v", got1, tt.want1)
			}
			out := w.String()
			if codes := extractCodes(out); !reflect.DeepEqual(codes, tt.wantCodes) {
				t.Logf("out = %v", out)
				t.Errorf("codes = %v, want %v", codes, tt.wantCodes)
			}
		})
	}
}
