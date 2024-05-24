package smtpd

import (
	"qmail-smtpd/internal/scan"
	"strings"
	"testing"
)

func Test_addrparse(t *testing.T) {
	tests := []struct {
		name  string
		arg   string
		want  string
		want1 bool
	}{
		{
			`[+]TO:<vasya@pupkin>`,
			`TO:<vasya@pupkin>`,
			"vasya@pupkin",
			true,
		},
		{
			`[+]TO:<vasya>`,
			`TO:<vasya>`,
			"vasya",
			true,
		},
		{
			`[+]TO:<>`,
			`TO:<>`,
			"",
			true,
		},
		{
			`[!]TO:<too_long@...>`,
			`TO:<too_long@` + strings.Repeat("x",900)+ `>`,
			"",
			false,
		},
		{
			`[+]TO: <vasya@pupkin>`,
			`TO: <vasya@pupkin>`,
			"vasya@pupkin",
			true,
		},
		{
			`[+] TO:<vasya@pupkin>`,
			` TO:<vasya@pupkin>`,
			"vasya@pupkin",
			true,
		},
		{
			`[+]TO:<vasya@pupkin> `,
			`TO:<vasya@pupkin> `,
			"vasya@pupkin",
			true,
		},
		{
			`[+]TO:<@mail.ru:vasya@pupkin>`,
			`TO:<@mail.ru:vasya@pupkin>`,
			"vasya@pupkin",
			true,
		},
		{
			`[!]TO:<@mail.ru>`,
			`TO:<@mail.ru>`,
			"",
			false,
		},
		{
			`[+]from:<"vasya pupkin"@mail.ru>`,
			`from:<"vasya pupkin"@mail.ru>`,
			"vasya pupkin@mail.ru",
			true,
		},
		{
			`[+]from:<vasya\ pupkin@mail.ru>`,
			`from:<vasya\ pupkin@mail.ru>`,
			"vasya pupkin@mail.ru",
			true,
		},
		{
			`[+]<vasya@pupkin>`,
			`<vasya@pupkin>`,
			"vasya@pupkin",
			true,
		},
		{
			`[+] <vasya@pupkin>`,
			` <vasya@pupkin>`,
			"vasya@pupkin",
			true,
		},
		{
			`[+]<vasya@pupkin> `,
			`<vasya@pupkin> `,
			"vasya@pupkin",
			true,
		},
		{
			`[!]vasya@pupkin`,
			`vasya@pupkin`,
			"",
			false,
		},
		{
			`[!] vasya@pupkin`,
			` vasya@pupkin`,
			"",
			false,
		},
		{
			`[+]from:vasya@pupkin`,
			`from:vasya@pupkin`,
			"vasya@pupkin",
			true,
		},
		{
			`[+] from:vasya@pupkin`,
			` from:vasya@pupkin`,
			"vasya@pupkin",
			true,
		},
		{
			`[+]from:vasya@pupkin `,
			`from:vasya@pupkin `,
			"vasya@pupkin",
			true,
		},
		{
			`[+] from: vasya@pupkin `,
			` from: vasya@pupkin `,
			"vasya@pupkin",
			true,
		},
		{
			`[+]from:"vasya pupkin"@mail.ru`,
			`from:"vasya pupkin"@mail.ru`,
			"vasya pupkin@mail.ru",
			true,
		},
		{
			`[+]from:vasya\ pupkin@mail.ru`,
			`from:vasya\ pupkin@mail.ru`,
			"vasya pupkin@mail.ru",
			true,
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := addrparse(tt.arg)
			if got != tt.want {
				t.Errorf("addrparse() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("addrparse() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

type alwaysIs struct{}
func (a alwaysIs) Is(scan.IPAddress) bool { return true }

type alwaysNotIs struct{}
func (a alwaysNotIs) Is(scan.IPAddress) bool { return false }

func Test_replaceLocalIP(t *testing.T) {
	type args struct {
		addr string
		host string
		ipme IPMe
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"[+] vasya@[192.168.69.69]",
			args{"vasya@[192.168.69.69]", "example.com", alwaysIs{}},
			"vasya@example.com",

		},
		{
			"[-] vasya@[192.168.69.69]",
			args{"vasya@[192.168.69.69]", "example.com", alwaysNotIs{}},
			"vasya@[192.168.69.69]",

		},
		{
			"[-] vasya@192.168.69.69",
			args{"vasya@192.168.69.69", "example.com", alwaysIs{}},
			"vasya@192.168.69.69",

		},
		{
			"[-] vasya@example.org",
			args{"vasya@example.org", "example.com", alwaysIs{}},
			"vasya@example.org",

		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replaceLocalIP(tt.args.addr, tt.args.host, tt.args.ipme); got != tt.want {
				t.Errorf("replaceLocalIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
