package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qmail-smtpd/internal/auth"
	"qmail-smtpd/internal/cdb"
	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/server"
	"qmail-smtpd/internal/smtpd"
	"qmail-smtpd/internal/tcprules"
	"qmail-smtpd/internal/todo"
)

type cdbAdapter struct{ cdb *cdb.CDB }

func (c cdbAdapter) Do(fn func(tcprules.Getter) error) error {
	return c.cdb.Do(func(q *cdb.Query) error { return fn(q) })
}

type smtpdAdapter struct{ *smtpd.Server }

func (h smtpdAdapter) Handle(ctx context.Context, env env.Env, conn net.Conn) error {
	return h.Run(ctx, conn, env)
}

var (
	rulesFile      = flag.String("x", "", "tcp rules file")
	lookupRemote   = flag.Bool("h", false, "lookup remote host name")
	paranoid       = flag.Bool("p", false, "paranoid check remote host name")
	maxConnections = flag.Int("c", 0, "maximum connections")
	serverAddr     = flag.String("addr", ":25", "addres to bind server")
	authFQDN       = flag.String("fqdn", "", "used only for SMTP AUTH (default control/me)")
	helpLong       = flag.Bool("help", false, "Show this help message")
)

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, "Usage: %s [OPTIONS] [AUTH_PROG] [AUTH_OPTIOS]\n", os.Args[0])
	fmt.Fprintf(out, "\nOptions:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *helpLong {
		usage()
		return
	}

	if *maxConnections < 0 {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "maximum connections number must be positive\n\n")
		usage()
		os.Exit(1)
	}

	slog.SetDefault(todo.NewLogger())

	var tcpRules *tcprules.Rules
	if *rulesFile != "" {
		db, err := cdb.Open(*rulesFile)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		tcpRules = tcprules.New(cdbAdapter{db})
	}

	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}

	cfg, err := todo.LoadQmailConfig()
	if err != nil {
		log.Fatalf("can't load qmail config: %v", err)
	}

	if args := flag.Args(); len(args) > 0 {
		if *authFQDN != "" {
			cfg.AuthFQDN = *authFQDN
		}
		cfg.Auth = auth.NewVchkpwCommand(args[0], args[1:]...)
	}

	smtpd := smtpd.NewServer(cfg)

	handler := server.Handler{
		LocalHost:     cfg.Me,
		LookupRemote:  *lookupRemote,
		LookupTimeout: 10 * time.Second,
		Paranoid:      *paranoid,
		TCPRules:      tcpRules,
		Handler:       smtpdAdapter{smtpd},
	}

	server := server.Server{
		MaxConnections: *maxConnections,
		Handler:        handler,
	}

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

	if err := server.Serve(listner); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Fatalf("server failed: %v", err)
	}

	if err := server.WaitAllConnections(30 * time.Second); err != nil {
		log.Fatalf("shutdown failed: %v", err)
	}
}
