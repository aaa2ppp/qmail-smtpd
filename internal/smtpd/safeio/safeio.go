package safeio

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"time"
)

type safeReader struct {
	conn    net.Conn
	timeout time.Duration
}

func (r *safeReader) Read(b []byte) (int, error) {
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

type SafeIO struct {
	conn       net.Conn
	timeout    time.Duration
	r          *bufio.Reader
	w          *bufio.Writer
	logIn      *logWriter
	logOut     *logWriter
	logEnabled bool
	tlsEnabled bool
	autoFlush  bool
}

func New(conn net.Conn, logger io.Writer, timeout time.Duration) *SafeIO {
	conn.SetDeadline(time.Time{}) // disable all deadlines

	w := bufio.NewWriter(&safeWriter{
		conn:    conn,
		timeout: timeout,
	})

	r := bufio.NewReader(&safeReader{
		conn:    conn,
		timeout: timeout,
	})

	var logIn, logOut *logWriter
	if logger != nil {
		logIn = newLogWriter(logger, "=> ")
		logOut = newLogWriter(logger, "<= ")
	}

	return &SafeIO{
		conn:       conn,
		timeout:    timeout,
		r:          r,
		w:          w,
		logIn:      logIn,
		logOut:     logOut,
		logEnabled: logger != nil,
		autoFlush:  true,
	}
}

// ReadLine reads a line from the client, up to and including '\n'.
// It returns a string (not []byte) with '\r\n' or '\n' stripped.
// Unlike bufio.Reader.ReadLine, this method is safe to use and does not
// return slices of an internal buffer.
func (cio *SafeIO) ReadLine() (string, error) {
	if cio.autoFlush {
		if err := cio.Flush(); err != nil {
			return "", err
		}
	}

	lineBytes, err := cio.r.ReadSlice('\n')

	// logging input data, if any, before checking for errors
	if cio.logEnabled && cio.logIn != nil && len(lineBytes) > 0 {
		cio.logIn.WriteString(string(lineBytes))
		cio.logIn.Flush()
	}

	if err != nil {
		return "", err
	}

	// cut out \n
	n := len(lineBytes) - 1

	// cut out '\r' if exists
	if n >= 1 && lineBytes[n-1] == '\r' {
		n--
	}

	return string(lineBytes[:n]), nil
}

func (cio *SafeIO) ReadByte() (byte, error) {
	return cio.r.ReadByte()
}

func (cio *SafeIO) WriteString(s string) (int, error) {
	if cio.logOut != nil && cio.logEnabled {
		cio.logOut.WriteString(s)
	}
	return cio.w.WriteString(s)
}

func (cio *SafeIO) WriteByte(c byte) error {
	return cio.w.WriteByte(c)
}

func (cio *SafeIO) Flush() error {
	if cio.logOut != nil && cio.logEnabled {
		cio.logOut.Flush()
	}
	return cio.w.Flush()
}

// AutoFlush enables/disables automatic flushing of the output buffer
// before each ReadLine call. Returns old value.
//
// It does not affect ReadByte or WriteByte.
//
// Auto flushing enabled by default. After server and client agreed
// PIPELINING, AutoFlush should be disabled, and flushing should be
// done explicitly.
func (cio *SafeIO) AutoFlush(enable bool) bool {
	old := cio.autoFlush
	cio.autoFlush = enable
	return old
}

// Logging enables/disables logging of session dialog in ReadLine and
// WriteString. Returns old value.
//
// It does not affect ReadByte or WriteByte (always no logging).
func (cio *SafeIO) Logging(enabled bool) bool {
	old := cio.logEnabled
	cio.logEnabled = enabled
	return old
}

func (cio *SafeIO) StartTLS(cfg *tls.Config) error {
	if cio.tlsEnabled {
		return errors.New("duplicate call StartTLS")
	}

	if err := cio.Flush(); err != nil {
		return err
	}

	conn := tls.Server(cio.conn, cfg)

	w := bufio.NewWriter(&safeWriter{
		conn:    conn,
		timeout: cio.timeout,
	})

	r := bufio.NewReader(&safeReader{
		conn:    conn,
		timeout: cio.timeout,
	})

	cio.conn = conn
	cio.r = r
	cio.w = w
	cio.tlsEnabled = true

	return nil
}

var (
	_ io.ByteReader   = &SafeIO{}
	_ io.ByteWriter   = &SafeIO{}
	_ io.StringWriter = &SafeIO{}
)
