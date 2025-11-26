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
func (sio *SafeIO) ReadLine() (string, error) {
	if sio.autoFlush {
		if err := sio.Flush(); err != nil {
			return "", err
		}
	}

	lineBytes, err := sio.r.ReadSlice('\n')

	// logging input data, if any, before checking for errors
	if sio.logEnabled && sio.logIn != nil && len(lineBytes) > 0 {
		sio.logIn.WriteString(string(lineBytes))
		sio.logIn.Flush()
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

func (sio *SafeIO) ReadByte() (byte, error) {
	return sio.r.ReadByte()
}

func (sio *SafeIO) WriteString(s string) (int, error) {
	if sio.logOut != nil && sio.logEnabled {
		sio.logOut.WriteString(s)
	}
	return sio.w.WriteString(s)
}

func (sio *SafeIO) WriteByte(c byte) error {
	return sio.w.WriteByte(c)
}

func (sio *SafeIO) Flush() error {
	if sio.logOut != nil && sio.logEnabled {
		sio.logOut.Flush()
	}
	return sio.w.Flush()
}

// AutoFlush enables/disables automatic flushing of the output buffer
// before each ReadLine call. Returns old value.
//
// It does not affect ReadByte or WriteByte.
//
// Auto flushing enabled by default. After server and client agreed
// PIPELINING, AutoFlush should be disabled, and flushing should be
// done explicitly.
func (sio *SafeIO) AutoFlush(enable bool) bool {
	old := sio.autoFlush
	sio.autoFlush = enable
	return old
}

// Logging enables/disables logging of session dialog in ReadLine and
// WriteString. Returns old value.
//
// It does not affect ReadByte or WriteByte (always no logging).
func (sio *SafeIO) Logging(enabled bool) bool {
	old := sio.logEnabled
	sio.logEnabled = enabled
	return old
}

func (sio *SafeIO) StartTLS(cfg *tls.Config) error {
	if sio.tlsEnabled {
		return errors.New("duplicate call StartTLS")
	}

	if err := sio.Flush(); err != nil {
		return err
	}

	conn := tls.Server(sio.conn, cfg)

	w := bufio.NewWriter(&safeWriter{
		conn:    conn,
		timeout: sio.timeout,
	})

	r := bufio.NewReader(&safeReader{
		conn:    conn,
		timeout: sio.timeout,
	})

	sio.conn = conn
	sio.r = r
	sio.w = w
	sio.tlsEnabled = true

	return nil
}

var (
	_ io.ByteReader   = &SafeIO{}
	_ io.ByteWriter   = &SafeIO{}
	_ io.StringWriter = &SafeIO{}
)
