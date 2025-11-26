package tcpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/tcprules"
)

type TCPRules interface {
	GetByIP(ip string) (tcprules.Result, error)
	GetByIPHost(ip, host string) (tcprules.Result, error)
}

type Handler struct {
	LocalHost     string // -l <localhost>
	LookupRemote  bool   // -h
	Paranoid      bool   // -p
	Resolver      Resolver
	LookupTimeout time.Duration
	TCPRules      TCPRules // -x <cdb>
	Handler       ConnectionHandler
}

func (h *Handler) Handle(ctx context.Context, env env.Env, conn net.Conn) error {
	localIP, err := extractIP(conn.LocalAddr())
	if err != nil {
		return fmt.Errorf("can't extract local ip: %w", err)
	}
	env.Set("TCPLOCALIP", localIP)

	remoteIP, err := extractIP(conn.RemoteAddr())
	if err != nil {
		return fmt.Errorf("can't extract remote ip: %w", err)
	}
	env.Set("TCPREMOTEIP", remoteIP)

	localHost, remoteHost := h.getHostNames(ctx, localIP, remoteIP)
	if err := ctx.Err(); err != nil {
		return err
	}

	if localHost != "" {
		env.Set("TCPLOCALHOST", localHost)
	}

	if remoteHost != "" {
		env.Set("TCPREMOTEHOST", remoteHost)
	}

	if h.TCPRules != nil {
		rule, err := h.TCPRules.GetByIPHost(remoteIP, remoteHost)
		if err != nil {
			return fmt.Errorf("can't get rule for %s: %w", remoteIP, err)
		}
		if !rule.Allow {
			return nil
		}
		env.Copy(rule.Env)
	}

	return h.Handler.Handle(ctx, env, conn)
}

func extractIP(addr net.Addr) (string, error) {
	if addr == nil {
		return "", errors.New("addr is <nil>")
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "", errors.New("can't split addr: " + addr.String())
	}
	return host, nil
}
