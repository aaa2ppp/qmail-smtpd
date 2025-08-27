package smtpd

import (
	"bufio"
	"log"
	"net"
	"os"
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
		deadline := time.Now().Add(r.timeout)
		if err := r.conn.SetReadDeadline(deadline); err != nil {
			if err != os.ErrNoDeadline {
				return 0, err
			}
			r.timeout = 0 // to avoid trying next time
		}
	}
	return r.conn.Read(b)
}

type safeWriter struct {
	conn    net.Conn
	timeout time.Duration
}

func (w *safeWriter) Write(b []byte) (int, error) {
	if w.timeout > 0 {
		deadline := time.Now().Add(w.timeout)
		if err := w.conn.SetWriteDeadline(deadline); err != nil {
			if err != os.ErrNoDeadline {
				return 0, err
			}
			w.timeout = 0 // to avoid trying next time
		}
	}
	return w.conn.Write(b)
}

func (d *Smtpd) initIO(ss *Session, conn net.Conn) {
	timeout := d.cfg.Timeout
	log.Printf("timeout: %v", timeout)
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	conn.SetDeadline(time.Time{})
	ss.ssin = bufio.NewReader(&safeReader{
		conn:    conn,
		timeout: timeout,
		flush:   ss.flush,
	})
	ss.ssout = bufio.NewWriter(&safeWriter{
		conn:    conn,
		timeout: timeout,
	})
	ss.conn = conn
}
