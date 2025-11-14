package server

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

type mockResolver struct {
	lookupAddrFn func(ctx context.Context, addr string) ([]string, error)
	lookupHostFn func(ctx context.Context, host string) ([]string, error)
}

func (m mockResolver) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	if m.lookupAddrFn != nil {
		return m.lookupAddrFn(ctx, addr)
	}
	return nil, errors.New("not implemented")
}

func (m mockResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if m.lookupHostFn != nil {
		return m.lookupHostFn(ctx, host)
	}
	return nil, errors.New("not implemented")
}

func TestParanoidLookupHosts_Success(t *testing.T) {
	r := mockResolver{
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			if host == "good.example.com" {
				return []string{"192.0.2.1"}, nil
			}
			return []string{"198.51.100.1"}, nil
		},
	}
	l := lookupCfg{resolver: r}

	hosts := []string{"bad.example.com", "good.example.com"}
	addr := "192.0.2.1"

	result, err := l.paranoidCheck(context.Background(), hosts, addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "good.example.com" {
		t.Errorf("expected good.example.com, got %q", result)
	}
}

func TestParanoidLookupHosts_NoMatch(t *testing.T) {
	r := mockResolver{
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			return []string{"203.0.113.1"}, nil // не совпадает с addr
		},
	}
	l := lookupCfg{resolver: r}

	hosts := []string{"a.com", "b.com"}
	addr := "192.0.2.1"

	result, err := l.paranoidCheck(context.Background(), hosts, addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected no match, got %q", result)
	}
}

func TestParanoidLookupHosts_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // сразу отменяем

	r := mockResolver{
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			<-ctx.Done() // имитируем долгий запрос
			return nil, ctx.Err()
		},
	}
	l := lookupCfg{resolver: r}

	hosts := []string{"slow.example.com"}
	addr := "192.0.2.1"

	result, err := l.paranoidCheck(ctx, hosts, addr)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result on cancel, got %q", result)
	}
}

func TestParanoidLookupHosts_GoroutineLeak(t *testing.T) {
	r := mockResolver{
		lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
			if host == "host1" {
				return []string{"192.0.2.1"}, nil // быстрый ответ
			}
			<-ctx.Done() // бесконечно долгий DNS
			return nil, ctx.Err()
		},
	}
	l := lookupCfg{resolver: r}

	initialGoroutines := runtime.NumGoroutine()

	hosts := []string{"host1", "host2", "host3"}
	_, _ = l.paranoidCheck(context.Background(), hosts, "192.0.2.1")

	// Даем время на cleanup
	time.Sleep(10 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines > initialGoroutines {
		t.Errorf("ГОРУТИНЫ УТЕКАЮТ! Было: %d, стало: %d",
			initialGoroutines, finalGoroutines)
	}
}

func TestLookupConnAddrs(t *testing.T) {
	tests := []struct {
		name     string // description of this test case
		cfg      lookupCfg
		resolver Resolver
		localIP  string
		remoteIP string
		want     string
		want2    string
	}{
		{
			"lookup success",
			lookupCfg{lookupRemote: true},
			&mockResolver{
				lookupAddrFn: func(ctx context.Context, addr string) ([]string, error) {
					switch addr {
					case "192.168.0.1":
						return []string{"local.host"}, nil
					case "192.168.0.2":
						return []string{"remote.host"}, nil
					}
					return nil, nil
				},
			},
			"192.168.0.1",
			"192.168.0.2",
			"local.host",
			"remote.host",
		},
		{
			"lookup success (filterd error)",
			lookupCfg{lookupRemote: true},
			&mockResolver{
				lookupAddrFn: func(ctx context.Context, addr string) ([]string, error) {
					switch addr {
					case "192.168.0.1":
						return []string{"local.host"}, errors.New("bad names filtered")
					case "192.168.0.2":
						return []string{"remote.host"}, errors.New("bad names filtered")
					}
					return nil, nil
				},
			},
			"192.168.0.1",
			"192.168.0.2",
			"local.host",
			"remote.host",
		},
		{
			"lookup failed",
			lookupCfg{lookupRemote: true},
			&mockResolver{
				lookupAddrFn: func(ctx context.Context, addr string) ([]string, error) {
					switch addr {
					case "192.168.0.1":
						return []string{}, errors.New("NXDOMAIN")
					case "192.168.0.2":
						return nil, errors.New("NXDOMAIN")
					}
					return nil, nil
				},
			},
			"192.168.0.1",
			"192.168.0.2",
			"",
			"",
		},
		{
			"not lookup",
			lookupCfg{localHost: "my.host", lookupRemote: false},
			&mockResolver{
				lookupAddrFn: func(ctx context.Context, addr string) ([]string, error) {
					switch addr {
					case "192.168.0.1":
						return []string{"local.host"}, nil
					case "192.168.0.2":
						return []string{"remote.host"}, nil
					}
					return nil, nil
				},
			},
			"192.168.0.1",
			"192.168.0.2",
			"my.host",
			"",
		},
		{
			"paranoid success",
			lookupCfg{lookupRemote: true, paranoid: true},
			&mockResolver{
				lookupAddrFn: func(ctx context.Context, addr string) ([]string, error) {
					switch addr {
					case "192.168.0.1":
						return []string{"local.host"}, nil
					case "192.168.0.2":
						return []string{"remote.host"}, nil
					}
					return nil, nil
				},
				lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
					switch host {
					case "remote.host":
						return []string{"192.168.0.2"}, nil
					}
					return nil, nil
				},
			},
			"192.168.0.1",
			"192.168.0.2",
			"local.host",
			"remote.host",
		},
		{
			"paranoid failed",
			lookupCfg{lookupRemote: true, paranoid: true},
			&mockResolver{
				lookupAddrFn: func(ctx context.Context, addr string) ([]string, error) {
					switch addr {
					case "192.168.0.1":
						return []string{"local.host"}, nil
					case "192.168.0.2":
						return []string{"remote.host"}, nil
					}
					return nil, nil
				},
				lookupHostFn: func(ctx context.Context, host string) ([]string, error) {
					return nil, nil
				},
			},
			"192.168.0.1",
			"192.168.0.2",
			"local.host",
			"",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := tt.cfg
			l.resolver = tt.resolver
			got, got2 := l.hostNames(context.Background(), tt.localIP, tt.remoteIP)
			if got != tt.want {
				t.Errorf("lookupConnAddrs() = %v, want %v", got, tt.want)
			}
			if got2 != tt.want2 {
				t.Errorf("lookupConnAddrs() = %v, want %v", got2, tt.want2)
			}
		})
	}
}
