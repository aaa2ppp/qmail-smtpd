package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"qmail-smtpd/internal/auth"
	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/pipeconn"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/smtpd"
	"qmail-smtpd/internal/todo"
)

func main() {
	signal.Ignore(syscall.SIGPIPE)

	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}

	cfg, err := todo.LoadQmailConfig()
	if err != nil {
		log.Fatalf("can't load qmail config: %v", err)
	}

	if len(os.Args) > 2 {
		cfg.AuthFQDN = os.Args[1]
		cfg.Auth = auth.NewVchkpwCommand(os.Args[2], os.Args[3:]...)
	}

	smtpd := smtpd.NewServer(cfg)

	env := qmail.Env{
		LocalIP:    os.Getenv("TCPLOCALIP"),
		LocalHost:  os.Getenv("TCPLOCALHOST"),
		RemoteIP:   os.Getenv("TCPREMOTEIP"),
		RemoteHost: os.Getenv("TCPREMOTEHOST"),
		QmailQueue: os.Getenv("QMAILQUEUE"),
	}

	if s, ok := os.LookupEnv("RELAYCLIENT"); ok {
		env.RelayClient = s
		env.RelayClientOk = ok
	}

	if env.LocalIP == "" {
		log.Fatal("TCPLOCALIP must be defined")
	}
	if env.RemoteIP == "" {
		log.Fatal("TCPREMOTEIP must be defined")
	}

	conn := &pipeconn.Conn{
		Reader:   os.Stdin,
		Writer:   os.Stdout,
		LocalIP:  pipeconn.Addr(env.LocalIP),
		RemoteIP: pipeconn.Addr(env.RemoteIP),
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := smtpd.Run(ctx, conn, env); err != nil {
		log.Fatalf("run failed: %v", err)
	}
}
