package main

import (
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/smtp"
	"os"
	"time"

	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/conn"
	log1 "qmail-smtpd/internal/log"
	"qmail-smtpd/internal/smtpd"
)

type Conn struct {
	label      string
	input      []byte
	in         <-chan []byte
	out        chan<- []byte
	localAddr  conn.Addr
	remoteAddr conn.Addr
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (c *Conn) Read(b []byte) (n int, err error) {
	for len(c.input) == 0 {
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

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (c *Conn) Write(b []byte) (n int, err error) {
	c.out <- b
	return len(b), nil
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *Conn) Close() error {
	close(c.out)
	for range c.in {
	}
	return nil
}

// LocalAddr returns the local network address, if known.
func (c *Conn) LocalAddr() net.Addr {
	if c.localAddr == "" {
		return nil
	}
	return c.localAddr
}

// RemoteAddr returns the remote network address, if known.
func (c *Conn) RemoteAddr() net.Addr {
	if c.remoteAddr == "" {
		return nil
	}
	return c.remoteAddr
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c *Conn) SetDeadline(t time.Time) error {
	r_err := c.SetReadDeadline(t)
	w_err := c.SetWriteDeadline(t)
	return errors.Join(r_err, w_err)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

type logAdapter struct {
	log1.Writer
}

func (a *logAdapter) WithPrefix(prefix string) smtpd.LogWriter {
	return &logAdapter{log1.Writer{
		Out:    a.Out,
		Prefix: a.Prefix + prefix,
	}}
}

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
		localAddr:  "127.0.0.1",
		remoteAddr: "127.0.0.1",
	}
	clientConn := &Conn{
		label:      "<=",
		in:         s2c,
		out:        c2s,
		localAddr:  "127.0.0.1",
		remoteAddr: "127.0.0.1",
	}

	cert, err := tls.LoadX509KeyPair("control/servercert.pem", "control/servercert.pem")
	if err != nil {
		log.Fatalf("LoadX509KeyPair: %v", err)
	}
	serv := &smtpd.Smtpd{
		Greeting:   "localhost",
		LocalIP:    "127.0.0.1",
		LocalHost:  "localhost",
		RemoteIP:   "127.0.0.1",
		RemoteHost: "localhost",
		Hostname:   "localhost",
		TLSConfig:  &tls.Config{Certificates: []tls.Certificate{cert}},
	}
	if _, ok := os.LookupEnv("SMTPLOG"); ok {
		serv.Log = &logAdapter{log1.Writer{
			Out: os.Stderr,
		}}
	}

	done := make(chan struct{})
	go func() {
		serv.Run(servConn)
		servConn.Close()
		done <- struct{}{}
	}()

	client, err := smtp.NewClient(clientConn, "localhost")
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}
	_ = client
	// if err := client.Hello("localhost"); err != nil {
	// 	log.Fatalf("client.Hello: %v", err)
	// }
	if err := client.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		log.Fatalf("client.StartTLS: %v", err)
	}
	if err := client.Quit(); err != nil {
		log.Fatalf("client.Quit: %v", err)
	}
	<-done
}
