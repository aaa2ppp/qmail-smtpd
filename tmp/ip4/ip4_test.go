package ip4

import (
	"testing"
)

func Test_parseCIDR(t *testing.T) {
	type args struct {
		cidr string
	}
	tests := []struct {
		name     string
		args     args
		wantIp4  uint32
		wantMask uint32
		wantErr  bool
	}{
		{
			"1.2.3.4",
			args{"1.2.3.4"},
			0x01020304,
			0xffffffff,
			false,
		},
		{
			"1.2.3.4/24",
			args{"1.2.3.4/24"},
			0x01020304,
			0xffffff00,
			false,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIp4, gotMask, err := parseCIDR(tt.args.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCIDR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIp4 != tt.wantIp4 {
				t.Errorf("parseCIDR() gotIp4 = %v, want %v", gotIp4, tt.wantIp4)
			}
			if gotMask != tt.wantMask {
				t.Errorf("parseCIDR() gotMask = %v, want %v", gotMask, tt.wantMask)
			}
		})
	}
}

func TestIsAllowed(t *testing.T) {
	type args struct {
		rules [][2]string
		ip    string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"1",
			args{
				[][2]string{
					{"192.168.1.0/24", "ALLOW"},
					{"10.0.0.0/16", "DENY"},
					{"8.8.8.8", "ALLOW"},

				},
				"192.168.1.10",
			},
			true,
		},
		{
			"2",
			args{
				[][2]string{
					{"192.168.1.0/24", "ALLOW"},
					{"10.0.0.0/16", "DENY"},
					{"8.8.8.8", "ALLOW"},

				},
				"10.0.0.10",
			},
			false,
		},
		{
			"3",
			args{
				[][2]string{
					{"192.168.1.0/24", "ALLOW"},
					{"10.0.0.0/16", "DENY"},
					{"8.8.8.8", "ALLOW"},

				},
				"192.168.2.10",
			},
			false,
		},
		{
			"4",
			args{
				[][2]string{
					{"192.168.1.0/24", "ALLOW"},
					{"10.0.0.0/16", "DENY"},
					{"8.8.8.8", "ALLOW"},

				},
				"8.8.8.8/16",
			},
			true,
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAllowed(tt.args.rules, tt.args.ip); got != tt.want {
				t.Errorf("IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
