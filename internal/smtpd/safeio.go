package smtpd

import (
	"bufio"
	"io"
	"net"
	"time"
)

type innerReader interface {
	io.Reader
	SetReadDeadline(time.Time) error
}

type innerWriter interface {
	io.Writer
	SetWriteDeadline(time.Time) error
}

type safeReader struct {
	inner   innerReader
	timeout time.Duration
	flush   func()
}

func (sr *safeReader) Read(b []byte) (int, error) {
	if sr.flush != nil {
		sr.flush()
	}
	if sr.timeout != 0 {
		sr.inner.SetReadDeadline(time.Now().Add(sr.timeout))
	}
	return sr.inner.Read(b)
}

type safeWriter struct {
	inner   innerWriter
	timeout time.Duration
}

func (sr *safeWriter) Write(b []byte) (int, error) {
	if sr.timeout != 0 {
		sr.inner.SetWriteDeadline(time.Now().Add(sr.timeout))
	}
	return sr.inner.Write(b)
}

func (d *Smtpd) safeio_init(conn net.Conn) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	conn.SetDeadline(time.Time{})
	d.ssin = bufio.NewReader(&safeReader{inner: conn, timeout: timeout, flush: d.flush})
	d.ssout = bufio.NewWriter(&safeWriter{inner: conn, timeout: timeout})
	d.conn = conn
}
