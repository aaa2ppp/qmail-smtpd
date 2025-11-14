package server

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"qmail-smtpd/internal/logger"
)

type Resolver interface {
	LookupAddr(context.Context, string) ([]string, error)
	LookupHost(context.Context, string) ([]string, error)
}

type lookupCfg struct {
	localHost     string
	lookupRemote  bool
	paranoid      bool
	lookupTimeout time.Duration
	resolver      Resolver
}

// paranoidCheck returns a host from the list whose address matches the specified one.
// Otherwise, it returns an empty string. Returns any lookup errors.
// The calling code should always check the returned host name before error.
func (r *lookupCfg) paranoidCheck(ctx context.Context, hosts []string, addr string) (string, error) {
	type response struct {
		host string
		err  error
	}

	// ставим отмену контекста на случай, когда найдено совпадение, чтобы сообщить
	// незавершенным запросам, что результат больше не нужен и надо завершиться
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resps := make(chan response, len(hosts))

	for _, host := range hosts {
		go func(host string) {
			addrs, err := r.resolver.LookupHost(ctx, host)
			if slices.Contains(addrs, addr) {
				resps <- response{host, err}
			} else {
				resps <- response{"", err}
			}
		}(host)
	}

	var errs []error

	// читаем ответы всех запросов пока не найдено совпадение, не будут прочитаны все ответы
	// или пока контекст не отменен снаружи
	for range len(hosts) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case r := <-resps:
			if r.err != nil {
				errs = append(errs, r.err)
			}
			if r.host != "" {
				return r.host, errors.Join(errs...)
			}
		}
	}

	return "", errors.Join(errs...)
}

// hostNames returns host names for connection addresses. It may return empty result if name is not found or not lookup configured.
func (r *lookupCfg) hostNames(ctx context.Context, localIP, remoteIP string) (localHost, remoteHost string) {
	logger := func() *slog.Logger { return logger.FromContext(ctx).With("op", "hostNames") }

	localHost = r.localHost

	if timeout := r.lookupTimeout; timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var wg sync.WaitGroup

	if localHost == "" && localIP != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hosts, err := r.resolver.LookupAddr(ctx, localIP)
			if len(hosts) == 0 {
				if err != nil && !errors.Is(err, context.Canceled) {
					logger().Warn("can't lookup local address", "error", err, "addr", localIP)
				}
				return
			}
			localHost = hosts[0]
		}()
	}

	if r.lookupRemote && remoteIP != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hosts, err := r.resolver.LookupAddr(ctx, remoteIP)
			if len(hosts) == 0 {
				if err != nil && !errors.Is(err, context.Canceled) {
					logger().Warn("can't lookup remote address", "error", err, "addr", remoteIP)
				}
				return
			}
			if r.paranoid {
				host, err := r.paranoidCheck(ctx, hosts, remoteIP)
				if host == "" {
					if err != nil && !errors.Is(err, context.Canceled) {
						logger().Warn("can't check remote host", "error", err)
					}
					return
				}
				remoteHost = host
				return
			}
			remoteHost = hosts[0]
		}()
	}

	wg.Wait()

	return localHost, remoteHost
}
