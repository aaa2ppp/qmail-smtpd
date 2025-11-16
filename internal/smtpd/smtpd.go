package smtpd

import (
	"cmp"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"strconv"
	"time"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/smtpd/interfaces"
	"qmail-smtpd/internal/smtpd/safeio"
)

const proto = "ESMTP"

type (
	AddrMatcher = interfaces.AddrMatcher
	IPMe        = interfaces.IPMe
	Qmail       = interfaces.Qmail
	Queue       = interfaces.Queue
	LogWriter   = interfaces.LogWriter
)

const (
	MaxHops        = 100
	DefaultTimeout = 1200 * time.Second // WTF: why so many?
)

var ErrClientQuit = errors.New("client quit")

type Config struct {
	Me          string
	Greeting    string
	Timeout     time.Duration
	LocalIPHost string
	RcptHosts   AddrMatcher
	BadMailFrom AddrMatcher
	MbxHosts    AddrMatcher
	IPMe        IPMe
	Qmail       Qmail
	AuthFQDN    string // from argv[1] or -fqdn; used only for SMTP AUTH
	Auth        Authenticator
	TLSConfig   *tls.Config
	Logger      LogWriter
	Databytes   int
}

type Server struct {
	cfg      *Config
	cmdTable map[string]command
}

func NewServer(cfg *Config) *Server {
	d := &Server{cfg: cfg}
	d.cmdTable = newCommandTable(d)
	return d
}

func (d *Server) newSession(ctx context.Context, conn net.Conn, env env.Env) *session {
	env.Set("PROTO", proto)

	var ioLogger LogWriter
	if d.cfg.Logger != nil {
		ioLogger = d.cfg.Logger.WithPrefix(conn.RemoteAddr().String() + ": ")
	}
	io := safeio.New(conn, ioLogger, d.cfg.Timeout)

	// RFC требует FQDN или адрес, но для идентификации хоста предпочтительно FQDN.
	local := cmp.Or(env.Get("TCPLOCALHOST"), env.Get("TCPLOCALIP"), "unknown")

	remoteIP := cmp.Or(env.Get("TCPREMOTEIP"), "unknown")
	remoteHost := cmp.Or(env.Get("TCPREMOTEHOST"), "unknown")

	databytes := d.cfg.Databytes
	if v, err := strconv.Atoi(env.Get("DATABYTES")); err == nil && v >= 0 {
		databytes = v
	}

	return &session{
		ctx:        ctx,
		env:        env,
		SafeIO:     io,
		proto:      proto,
		local:      local,
		remoteIP:   remoteIP,
		remoteHost: remoteHost,
		databytes:  databytes,
	}
}

func (d *Server) run(ss *session) error {
	d.smtp_greet(ss, "220 ")
	ss.out(" ")
	ss.out(ss.proto)
	ss.out("\r\n")
	// TODO: здесь нужен Flush()?

	err := d.commandLoop(ss) // always return error
	if err == ErrClientQuit {
		return nil
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		d.err_timeout(ss)
		ss.Flush()
	}

	return err
}

func (d *Server) Run(ctx context.Context, conn net.Conn, env env.Env) error {
	ss := d.newSession(ctx, conn, env)
	d.dohelo(ss, ss.remoteHost)
	d.resetAuthorized(ss)
	return d.run(ss)
}
