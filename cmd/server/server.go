package main

import (
	"errors"
	"flag"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qmail-smtpd/internal/cdb"
	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/server"
	"qmail-smtpd/internal/smtpd"
	"qmail-smtpd/internal/tcprules"
	"qmail-smtpd/internal/todo"
)

var (
	cdbFileName = flag.String("x", "control/tcp.25.cdb", "tcp rules")
)

type cdbAdapter struct{ cdb *cdb.CDB }

func (c cdbAdapter) Do(fn func(tcprules.Getter) error) error {
	return c.cdb.Do(func(q *cdb.Query) error { return fn(q) })
}

func main() {
	slog.SetDefault(todo.NewLogger())

	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}

	if len(os.Args) < 2 {
		log.Fatal("Usage: server <addr:port>")
	}
	addr := os.Args[1]

	cfg, err := todo.LoadQmailConfig()
	if err != nil {
		log.Fatalf("can't load qmail config: %v", err)
	}
	smtpd := smtpd.NewServer(cfg)

	rulesDB, err := cdb.Open(*cdbFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer rulesDB.Close()

	handlerCfg := server.HandlerConfig{
		LocalHost:     cfg.Me,
		LookupRemote:  true,
		LookupTimeout: 10 * time.Second,
		TCPRules:      tcprules.New(cdbAdapter{rulesDB}),
	}
	handler := server.NewHandler(
		handlerCfg,
		smtpd,
	)

	serverCfg := server.Config{
		MaxConnections: 10,
	}
	server := server.New(
		serverCfg,
		handler,
	)

	listner, err := net.Listen("tcp", addr)
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

	if err := server.Wait(30 * time.Second); err != nil {
		log.Fatalf("shutdown failed: %v", err)
	}
}
