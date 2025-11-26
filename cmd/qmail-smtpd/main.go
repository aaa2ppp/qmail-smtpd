package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"qmail-smtpd/internal/auth"
	"qmail-smtpd/internal/cdb"
	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/control"
	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/pipeconn"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/smtpd"
	"qmail-smtpd/internal/smtpd/filters"
	"qmail-smtpd/internal/tcprules"
	"qmail-smtpd/internal/tcpserver"
)

type cdbAdapter struct{ cdb *cdb.CDB }

func (c cdbAdapter) Do(fn func(tcprules.Getter) error) error {
	return c.cdb.Do(func(q *cdb.Query) error { return fn(q) })
}

var (
	localHost    = flag.String("l", "", "localhost name")
	rulesFile    = flag.String("x", "", "tcp rules file")
	lookupRemote = flag.Bool("h", false, "lookup remote host name")
	paranoid     = flag.Bool("p", false, "paranoid check remote host name")
	maxConns     = flag.Int("c", 0, "maximum connections")
	serverAddr   = flag.String("addr", "", "addres to bind server")
	authFQDN     = flag.String("fqdn", "", "used only for SMTP AUTH (default control/localiphost)")
	helpLong     = flag.Bool("help", false, "Show this help message")
)

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, `Usage as cli:
  %[1]s [AUTH_FQDN AUTH_PROG AUTH_OPTIONS]
Usage as server:
  %[1]s --addr=<HOST:PORT> [OPTIONS] [AUTH_PROG AUTH_OPTIONS]
Options:
`, filepath.Base(os.Args[0]))
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *helpLong {
		usage()
		return
	}

	if *serverAddr != "" {
		runServer()
	} else {
		runCLI()
	}
}

func runCLI() {
	signal.Ignore(syscall.SIGPIPE)

	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}

	smtpdCfg, err := buildSMTPServerConfig()
	if err != nil {
		log.Fatalf("can't load qmail config: %v", err)
	}

	if len(os.Args) > 2 {
		smtpdCfg.AuthFQDN = os.Args[1]
		smtpdCfg.Auth = auth.NewVchkpwCommand(os.Args[2], os.Args[3:]...)
	}

	env := env.New(os.Environ())
	for _, name := range []string{"TCPLOCALIP", "TCPREMOTEIP"} {
		if env.Get(name) == "" {
			log.Fatalf("%s must be defined", name)
		}
	}

	conn := &pipeconn.Conn{
		Reader:   os.Stdin,
		Writer:   os.Stdout,
		LocalIP:  pipeconn.Addr(env.Get("TCPLOCALIP")),
		RemoteIP: pipeconn.Addr(env.Get("TCPREMOTEIP")),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	smtpServer := smtpd.NewServer(smtpdCfg)
	if err := smtpServer.Handle(ctx, env, conn); err != nil {
		log.Fatalf("run failed: %v", err)
	}
}

func runServer() {
	if *maxConns < 0 {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "maximum connections number must be positive\n\n")
		usage()
		os.Exit(1)
	}

	slog.SetDefault(newLogger())

	var tcpRules tcpserver.TCPRules
	if *rulesFile != "" {
		log.Printf("open %s", *rulesFile)
		db, err := cdb.Open(*rulesFile)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		tcpRules = tcprules.New(cdbAdapter{db})
	}

	var (
		vchkpwPath string
		vchkpwArgs []string
	)
	if args := flag.Args(); len(args) > 0 {
		if p, err := filepath.Abs(args[0]); err == nil {
			vchkpwPath = p
		} else {
			log.Fatal(err)
		}
		vchkpwArgs = args[1:]
	}

	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}

	smtpdCfg, err := buildSMTPServerConfig()
	if err != nil {
		log.Fatalf("can't load qmail config: %v", err)
	}

	if vchkpwPath != "" {
		if *authFQDN != "" {
			smtpdCfg.AuthFQDN = *authFQDN
		}
		smtpdCfg.Auth = auth.NewVchkpwCommand(vchkpwPath, vchkpwArgs...)
	}

	smtpServer := smtpd.NewServer(smtpdCfg)

	handler := tcpserver.Handler{
		LocalHost:     *localHost,
		LookupRemote:  *lookupRemote,
		LookupTimeout: 10 * time.Second,
		Paranoid:      *paranoid,
		TCPRules:      tcpRules,
		Handler:       smtpServer,
	}

	tcpServer := tcpserver.Server{
		MaxConns: *maxConns,
		Handler:  handler,
	}

	log.Printf("server listen at %s", *serverAddr)
	listner, err := net.Listen("tcp", *serverAddr)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		s := <-c
		log.Println("got signal:", s)
		listner.Close()
	}()

	if err := tcpServer.Serve(listner); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Fatalf("server failed: %v", err)
	}

	if err := tcpServer.WaitAllConnections(30 * time.Second); err != nil {
		log.Fatalf("shutdown failed: %v", err)
	}
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch s := os.Getenv("LOG_LEVEL"); {
	case strings.EqualFold(s, "DEBUG"):
		level = slog.LevelDebug
	case strings.EqualFold(s, "INFO"):
		level = slog.LevelInfo
	case strings.EqualFold(s, "WARN"):
		level = slog.LevelWarn
	case strings.EqualFold(s, "ERROR"):
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

type qmailAdapter struct{}

func (qa qmailAdapter) Begin(fromMail string, rcptTo []string, env env.Env) (smtpd.Queue, error) {
	return qmail.Begin(fromMail, rcptTo, env)
}

type rcptHostsDB struct{ cdb *cdb.CDB }

func (c rcptHostsDB) Do(fn func(filters.RcptHostsFinder) error) error {
	return c.cdb.Do(func(q *cdb.Query) error { return fn(q) })
}

func buildSMTPServerConfig() (smtpd.ServerConfig, error) {
	var cfg smtpd.ServerConfig

	if manager, err := config.NewManager(control.FileEngine{}); err != nil {
		return smtpd.ServerConfig{}, err
	} else {
		cfg.Manager = manager
	}

	cfg.Qmail = qmailAdapter{}

	if cert, err := tls.LoadX509KeyPair("control/servercert.pem", "control/servercert.pem"); err != nil {
		slog.Debug("load X509 key pair failed", "error", err)
	} else {
		cfg.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}

	if db, err := cdb.Open("control/morercpthosts.cdb"); err != nil {
		if !os.IsNotExist(err) {
			return smtpd.ServerConfig{}, err
		}
	} else {
		cfg.RcptHostsDB = rcptHostsDB{db}
	}

	return cfg, nil
}
