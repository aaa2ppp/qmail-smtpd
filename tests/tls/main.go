package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/smtp"
	"os"
	"time"

	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/control"
	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/pipeconn"
	"qmail-smtpd/internal/smtpd"
)

type Conn struct {
	label      string
	input      []byte
	in         <-chan []byte
	out        chan<- []byte
	localAddr  pipeconn.Addr
	remoteAddr pipeconn.Addr
}

func (c *Conn) Read(b []byte) (n int, err error) {
	if len(c.input) == 0 {
		input, ok := <-c.in
		if !ok {
			return 0, io.EOF
		}
		c.input = input
	}

	n = min(len(b), len(c.input))

	log.Printf("%s %s", c.label, c.input[:n])

	copy(b, c.input[:n])
	c.input = c.input[n:]
	return n, nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if len(b) != 0 {
		c.out <- b
	}
	return len(b), nil
}

func (c *Conn) Close() error {
	close(c.out)
	for range c.in {
	}
	return nil
}

func (c *Conn) LocalAddr() net.Addr                { return c.localAddr }
func (c *Conn) RemoteAddr() net.Addr               { return c.remoteAddr }
func (c *Conn) SetDeadline(t time.Time) error      { return nil }
func (c *Conn) SetReadDeadline(t time.Time) error  { return nil }
func (c *Conn) SetWriteDeadline(t time.Time) error { return nil }

var _ net.Conn = &Conn{}

func main() {
	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatalf("Chdir: %v", err)
	}

	c2s := make(chan []byte)
	s2c := make(chan []byte)

	servConn := &Conn{
		label:      "=>",
		in:         c2s,
		out:        s2c,
		localAddr:  "127.0.0.1:2525",
		remoteAddr: "127.0.0.1:61111",
	}

	clientConn := &Conn{
		label:      "<=",
		in:         s2c,
		out:        c2s,
		localAddr:  "127.0.0.1:61111",
		remoteAddr: "127.0.0.1:2525",
	}

	cert, err := tls.LoadX509KeyPair("control/servercert.pem", "control/servercert.pem")
	if err != nil {
		log.Fatalf("LoadX509KeyPair: %v", err)
	}

	env := env.New([]string{
		"TCPLOCALIP=127.0.0.1",
		"TCPLOCALHOST=localhost",
		"TCPREMOTEIP=127.0.0.1",
		"TCPREMOTEHOST=localhost",
	})

	cfgManager, err := config.NewManager(control.FileEngine{})
	if err != nil {
		log.Fatal(err)
	}

	cfg := smtpd.ServerConfig{
		Manager:   cfgManager,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	server := smtpd.NewServer(cfg)

	done := make(chan struct{})
	go func() {
		server.Handle(context.Background(), env, servConn)
		servConn.Close()
		done <- struct{}{}
	}()

	client, err := smtp.NewClient(clientConn, "localhost")
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	if err := client.Hello("localhost"); err != nil {
		log.Fatalf("client.Hello: %v", err)
	}
	if err := client.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		log.Fatalf("client.StartTLS: %v", err)
	}
	if err := client.Quit(); err != nil {
		log.Fatalf("client.Quit: %v", err)
	}
	<-done
}
