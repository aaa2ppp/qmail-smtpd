package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/tcprules"
)

type Runner interface {
	Run(context.Context, net.Conn, env.Env) error
}

type TCPRules interface {
	GetByIP(ip string) (tcprules.Result, error)
	GetByIPHost(ip, host string) (tcprules.Result, error)
}

// HandlerConfig. For more see man tcpserver.
type HandlerConfig struct {
	LocalHost        string // -l <localhost>
	LookupRemote     bool   // -h
	Paranoid         bool   // -p
	Resolver         Resolver
	LookupTimeout    time.Duration
	TCPRules         TCPRules // -x <cdb>
	ForbiddenMessage string
	WriteTimeout     time.Duration
}

type Handler struct {
	// rulesDB  *RulesDB
	runner           Runner
	rules            TCPRules
	lookup           *lookupCfg
	forbiddenMessage string
	writeTimeout     time.Duration
}

func NewHandler(cfg HandlerConfig, runner Runner) *Handler {
	return &Handler{
		runner: runner,
		lookup: &lookupCfg{
			localHost:     cfg.LocalHost,
			lookupRemote:  cfg.LookupRemote,
			paranoid:      cfg.Paranoid,
			lookupTimeout: cfg.LookupTimeout,
			resolver:      cmp.Or(cfg.Resolver, Resolver(net.DefaultResolver)),
		},
		rules:            cfg.TCPRules,
		forbiddenMessage: cfg.ForbiddenMessage,
		writeTimeout:     cfg.WriteTimeout,
	}
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

	localHost, remoteHost := h.lookup.hostNames(ctx, localIP, remoteIP)
	if err := ctx.Err(); err != nil {
		return err
	}

	if localHost != "" {
		env.Set("TCPLOCALHOST", localHost)
	}

	if remoteHost != "" {
		env.Set("TCPREMOTEHOST", remoteHost)
	}

	rule, err := h.rules.GetByIPHost(remoteIP, remoteHost)
	if err != nil {
		return fmt.Errorf("can't get rule for %s: %w", remoteIP, err)
	}
	if !rule.Allow {
		if h.forbiddenMessage != "" {
			if h.writeTimeout > 0 {
				conn.SetWriteDeadline(time.Now().Add(h.writeTimeout))
			}
			conn.Write([]byte(h.forbiddenMessage))
		}
		return nil
	}

	env.Copy(rule.Env)

	return h.runner.Run(ctx, conn, env)
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
