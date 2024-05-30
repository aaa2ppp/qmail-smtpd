package conn

import (
	"errors"
	"io"
	"net"
	"time"
)

var _ net.Conn = (*Conn)(nil)

type Addr string

func (a Addr) String() string {
	return string(a)
}

func (a Addr) Network() string {
	return "tcp"
}

type Reader interface {
	io.Reader
	io.Closer
	SetReadDeadline(t time.Time) error
}

type Writer interface {
	io.Writer
	io.Closer
	SetWriteDeadline(t time.Time) error
}

type Conn struct {
	Reader
	Writer
	LocalIP  Addr
	RemoteIP Addr
}

func (c *Conn) Close() error {
	r_err := c.Reader.Close()
	w_err := c.Writer.Close()
	return errors.Join(r_err, w_err)
}

func (c *Conn) LocalAddr() net.Addr {
	switch c.LocalIP {
	case "", "unknown":
		return nil
	}
	return c.LocalIP
}

func (c *Conn) RemoteAddr() net.Addr {
	switch c.RemoteIP {
	case "", "unknown":
		return nil
	}
	return c.RemoteIP
}

func (c *Conn) SetDeadline(t time.Time) error {
	r_err := c.SetReadDeadline(t)
	w_err := c.SetWriteDeadline(t)
	return errors.Join(r_err, w_err)
}
