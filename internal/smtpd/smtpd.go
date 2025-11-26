package smtpd

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"strconv"
	"time"
	"unsafe"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/logger"
	"qmail-smtpd/internal/smtpd/filters"
	"qmail-smtpd/internal/smtpd/safeio"
)

const (
	Proto          = "ESMTP"
	DefaultTimeout = 1200 * time.Second
)

type Config struct {
	Greeting    string
	LocalIPHost string
	Timeout     time.Duration
	Databytes   int
	BadMailFrom filters.ConstMap
	RcptHosts   filters.ConstMap
	MbxHosts    filters.ConstMap
	MbxZone     string
	OpenRealy   bool
	SMTPLog     bool
}

type ConfigManager interface {
	Get() *Config
}

type Qmail interface {
	Begin(mailForm string, rcptTo []string, env env.Env) (Queue, error)
}

type Queue interface {
	io.ByteWriter
	io.StringWriter
	Pid() int
	Rollback() error
	Commit() error
}

type ServerConfig struct {
	Manager     ConfigManager
	Qmail       Qmail
	Auth        Authenticator
	AuthFQDN    string
	TLSConfig   *tls.Config
	RcptHostsDB filters.RcptHostsDB
}

type Server struct {
	cfg      ServerConfig
	commands commands
}

func NewServer(cfg ServerConfig) *Server {
	commands := newSMTPCommands()
	return &Server{
		cfg:      cfg,
		commands: commands,
	}
}

func (srv *Server) Handle(ctx context.Context, env env.Env, conn net.Conn) error {
	env.Set("PROTO", Proto)
	ss := srv.setupSession(ctx, env, conn)
	return smtp_run(ss, srv.commands)
}

// void setup()
// {
//   char *x;
//   unsigned long u;

//   if (control_init() == -1) die_control();
//   if (control_rldef(&greeting,"control/smtpgreeting",1,(char *) 0) != 1)
//     die_control();
//   liphostok = control_rldef(&liphost,"control/localiphost",1,(char *) 0);
//   if (liphostok == -1) die_control();
//   if (control_readint(&timeout,"control/timeoutsmtpd") == -1) die_control();
//   if (timeout <= 0) timeout = 1;

//   if (rcpthosts_init() == -1) die_control();

//   bmfok = control_readfile(&bmf,"control/badmailfrom",0);
//   if (bmfok == -1) die_control();
//   if (bmfok)
//     if (!constmap_init(&mapbmf,bmf.s,bmf.len,0)) die_nomem();

//   if (control_readint(&databytes,"control/databytes") == -1) die_control();
//   x = env_get("DATABYTES");
//   if (x) { scan_ulong(x,&u); databytes = u; }
//   if (!(databytes + 1)) --databytes;

//   remoteip = env_get("TCPREMOTEIP");
//   if (!remoteip) remoteip = "unknown";
//   local = env_get("TCPLOCALHOST");
//   if (!local) local = env_get("TCPLOCALIP");
//   if (!local) local = "unknown";
//   remotehost = env_get("TCPREMOTEHOST");
//   if (!remotehost) remotehost = "unknown";
//   remoteinfo = env_get("TCPREMOTEINFO");
//   relayclient = env_get("RELAYCLIENT");
//   dohelo(remotehost);
// }

type logWriter struct {
	ctx context.Context
	log *slog.Logger
}

func (lw *logWriter) Write(b []byte) (int, error) {
	lw.log.Log(lw.ctx, slog.LevelDebug, unsafe.String(unsafe.SliceData(b), len(b)))
	return len(b), nil
}

func (srv *Server) setupSession(ctx context.Context, env env.Env, conn net.Conn) *session {
	cfg := srv.cfg.Manager.Get()

	var logWrtr io.Writer
	if cfg.SMTPLog {
		log := logger.FromContext(ctx)
		if log.Enabled(ctx, slog.LevelDebug) {
			logWrtr = &logWriter{ctx, log}
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	safeIO := safeio.New(conn, logWrtr, timeout)

	ss := &session{}
	ss.ctx = ctx
	ss.env = env
	ss.io = safeIO

	ss.qmail = srv.cfg.Qmail
	ss.auth = srv.cfg.Auth
	ss.authFQDN = srv.cfg.AuthFQDN
	if ss.authFQDN == "" {
		ss.authFQDN = cfg.LocalIPHost
	}
	ss.tlsConfig = srv.cfg.TLSConfig

	ss.proto = Proto
	ss.greeting = cfg.Greeting
	ss.localIPHost = cfg.LocalIPHost

	ss.badMailFrom = filters.NewBadMailFrom(cfg.BadMailFrom)
	ss.rcptHosts = filters.NewRcptHosts(cfg.RcptHosts, srv.cfg.RcptHostsDB)
	ss.noMailbox = filters.NewNoMailBox(cfg.MbxHosts, cfg.MbxZone, cfg.LocalIPHost, nil)

	ss.databytes = cfg.Databytes
	if i, err := strconv.Atoi(env.Get("DATABYTES")); err == nil && i >= 0 {
		ss.databytes = i
	}
	if ss.databytes <= 0 {
		ss.databytes = 0
	}

	ss.remoteIP = env.Get("TCPREMOTEIP")
	if ss.remoteIP == "" {
		ss.remoteIP = "unknown"
	}

	ss.local = env.Get("TCPLOCALHOST")
	if ss.local == "" {
		ss.local = env.Get("TCPLOCALIP")
	}
	if ss.local == "" {
		ss.local = "unknown"
	}

	ss.remoteHosts = env.Get("TCPREMOTEHOST")
	if ss.remoteHosts == "" {
		ss.remoteHosts = "unknown"
	}

	ss.remoteInfo = env.Get("TCPREMOTEINFO")
	ss.relaySuffix, ss.relayClient = env.Lookup("RELAYCLIENT")

	return ss
}
