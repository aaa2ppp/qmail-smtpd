package smtpd

import (
	"bufio"
	"net"
	"time"
)

type safeReader struct {
	conn    net.Conn
	timeout time.Duration
	flush   func() error
}

func (r *safeReader) Read(b []byte) (int, error) {
	if r.flush != nil {
		if err := r.flush(); err != nil {
			return 0, err
		}
	}
	if r.timeout > 0 {
		if err := r.conn.SetReadDeadline(time.Now().Add(r.timeout)); err != nil {
			return 0, err
		}
	}
	return r.conn.Read(b)
}

type safeWriter struct {
	conn    net.Conn
	timeout time.Duration
}

func (sr *safeWriter) Write(b []byte) (int, error) {
	if sr.timeout > 0 {
		sr.conn.SetWriteDeadline(time.Now().Add(sr.timeout))
	}
	return sr.conn.Write(b)
}

func (d *Smtpd) initIO(conn net.Conn) {
	timeout := d.Timeout
	if timeout < 0 {
		panic("Smtpd.initIO: timeout cannot be negative")
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	conn.SetDeadline(time.Time{})
	d.ssin = bufio.NewReader(&safeReader{
		conn:    conn,
		timeout: timeout,
		flush:   d.flush,
	})
	d.ssout = bufio.NewWriter(&safeWriter{
		conn:    conn,
		timeout: timeout,
	})
	d.conn = conn
}
