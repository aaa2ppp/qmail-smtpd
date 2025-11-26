package tcpserver

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"qmail-smtpd/internal/logger"
)

type Resolver interface {
	LookupAddr(context.Context, string) ([]string, error)
	LookupHost(context.Context, string) ([]string, error)
}

func lookupAddr(ctx context.Context, resolver Resolver, addr string, paranoid bool) (string, error) {
	hosts, err := resolver.LookupAddr(ctx, addr)
	if len(hosts) == 0 {
		return "", err
	}
	if !paranoid {
		return hosts[0], nil
	}

	host, err := paranoidCheck(ctx, resolver, hosts, addr)
	if host == "" {
		return "", err
	}
	return host, nil
}

// paranoidCheck returns a host from the list whose address matches the specified one.
// Otherwise, it returns an empty string. Returns any lookup errors.
// The calling code should always check the returned host name before error.
func paranoidCheck(ctx context.Context, resolver Resolver, hosts []string, addr string) (string, error) {
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
			addrs, err := resolver.LookupHost(ctx, host)
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

func (h *Handler) resolver() Resolver {
	if h.Resolver != nil {
		return h.Resolver
	}
	return net.DefaultResolver
}

// getHostNames returns host names for connection addresses.
// It may return empty result if name is not found or not lookup configured.
func (h *Handler) getHostNames(ctx context.Context, localIP, remoteIP string) (localHost, remoteHost string) {
	resolver := h.resolver()
	logger := func() *slog.Logger {
		return logger.FromContext(ctx).With("op", "Handler.getHostNames")
	}

	tasks := make([]func(), 0, 2)

	localHost = h.LocalHost
	if localHost == "" && localIP != "" {
		tasks = append(tasks, func() {
			host, err := lookupAddr(ctx, resolver, localIP, false)
			if err == nil {
				localHost = host
			} else if !errors.Is(err, context.Canceled) {
				logger().Warn("can't lookup local address", "error", err, "addr", localIP)
			}
		})
	}

	if h.LookupRemote && remoteIP != "" {
		tasks = append(tasks, func() {
			host, err := lookupAddr(ctx, resolver, remoteIP, h.Paranoid)
			if err == nil {
				remoteHost = host
			} else if !errors.Is(err, context.Canceled) {
				logger().Warn("can't lookup remote address", "error", err, "addr", remoteIP)
			}
		})
	}

	ctx, cancel := context.WithTimeout(ctx, cmp.Or(h.LookupTimeout, 10*time.Second))
	defer cancel()

	var wg sync.WaitGroup
	for i := len(tasks) - 1; i >= 0; i-- {
		if i == 0 {
			tasks[0]()
		} else {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				tasks[i]()
			}(i)
		}
	}
	wg.Wait()

	return localHost, remoteHost
}
