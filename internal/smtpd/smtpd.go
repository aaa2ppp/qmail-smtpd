package smtpd

import (
	"crypto/tls"
	"errors"
	"net"
	"os"
	"time"

	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/smtpd/interfaces"
	"qmail-smtpd/internal/smtpd/safeio"
)

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
	Greeting    string
	Timeout     time.Duration
	LocalIPHost string
	RcptHosts   AddrMatcher
	BadMailFrom AddrMatcher
	MbxHosts    AddrMatcher
	IPMe        IPMe
	Qmail       Qmail
	Hostname    string
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

func (d *Server) newSession(conn net.Conn, env qmail.Env) *session {
	var logger LogWriter
	if d.cfg.Logger != nil {
		logger = d.cfg.Logger.WithPrefix(env.RemoteIP + ": ")
	}
	return &session{
		SafeIO: safeio.New(conn, logger, d.cfg.Timeout),
		env:    env,
	}
}

func (d *Server) run(ss *session) error {
	d.smtp_greet(ss, "220 ")
	ss.out(" ")
	ss.out(ss.env.Proto)
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

func (d *Server) Run(conn net.Conn, env qmail.Env) error {
	env.Proto = "ESMTP"

	ss := d.newSession(conn, env)
	d.dohelo(ss, ss.env.RemoteHost)
	d.resetAuthorized(ss)

	return d.run(ss)
}
