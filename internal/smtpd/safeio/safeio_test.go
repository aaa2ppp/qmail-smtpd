package safeio

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"qmail-smtpd/internal/smtpd/interfaces"
)

// mockLogWriter implements interfaces.LogWriter
type mockLogWriter struct {
	buf *bytes.Buffer
}

func (m *mockLogWriter) WriteString(s string) (int, error) {
	return m.buf.WriteString(s)
}

func (m *mockLogWriter) Flush() error {
	return nil
}

func (m *mockLogWriter) WithPrefix(prefix string) interfaces.LogWriter {
	return &prefixedLogWriter{prefix: prefix, base: m}
}

type prefixedLogWriter struct {
	prefix string
	base   *mockLogWriter
}

func (p *prefixedLogWriter) WriteString(s string) (int, error) {
	return p.base.buf.WriteString(p.prefix + s)
}

func (p *prefixedLogWriter) Flush() error {
	return nil
}

func (p *prefixedLogWriter) WithPrefix(prefix string) interfaces.LogWriter {
	return &prefixedLogWriter{prefix: prefix, base: p.base}
}

// mockConn implements net.Conn minimally for testing
type mockConn struct {
	reader *strings.Reader
	writer *bytes.Buffer
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	return m.reader.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, errors.New("write on closed conn")
	}
	return m.writer.Write(b)
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// --- Тесты ---

func TestReadLine_Normal(t *testing.T) {
	conn := &mockConn{reader: strings.NewReader("HELO example.com\r\n"), writer: &bytes.Buffer{}}
	sio := New(conn, nil, 0)

	line, err := sio.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "HELO example.com" {
		t.Errorf("expected 'HELO example.com', got %q", line)
	}
}

func TestReadLine_LFOnly(t *testing.T) {
	conn := &mockConn{reader: strings.NewReader("QUIT\n"), writer: &bytes.Buffer{}}
	sio := New(conn, nil, 0)

	line, err := sio.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "QUIT" {
		t.Errorf("expected 'QUIT', got %q", line)
	}
}

func TestReadLine_EmptyLine(t *testing.T) {
	conn := &mockConn{reader: strings.NewReader("\r\n"), writer: &bytes.Buffer{}}
	sio := New(conn, nil, 0)

	line, err := sio.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "" {
		t.Errorf("expected empty string, got %q", line)
	}
}

func TestReadLine_NoNewline(t *testing.T) {
	conn := &mockConn{reader: strings.NewReader("INCOMPLETE"), writer: &bytes.Buffer{}}
	sio := New(conn, nil, 0)

	_, err := sio.ReadLine()
	if !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		t.Errorf("expected EOF or ErrBufferFull, got %v", err)
	}
}

func TestReadLine_Logging(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := &mockLogWriter{buf: logBuf}
	conn := &mockConn{reader: strings.NewReader("DATA\r\n"), writer: &bytes.Buffer{}}
	sio := New(conn, logger, 0)

	_, err := sio.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "=> DATA\r\n"
	if logBuf.String() != expected {
		t.Errorf("log mismatch:\nwant %q\ngot  %q", expected, logBuf.String())
	}
}

func TestWriteString_Logging(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := &mockLogWriter{buf: logBuf}
	conn := &mockConn{reader: strings.NewReader(""), writer: &bytes.Buffer{}}
	sio := New(conn, logger, 0)

	_, err := sio.WriteString("250 OK\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Logging happens immediately on WriteString
	expected := "<= 250 OK\r\n"
	if logBuf.String() != expected {
		t.Errorf("log mismatch:\nwant %q\ngot  %q", expected, logBuf.String())
	}
}

func TestAutoFlush(t *testing.T) {
	conn := &mockConn{reader: strings.NewReader("NOOP\r\n"), writer: &bytes.Buffer{}}
	sio := New(conn, nil, 0)

	// Disable auto-flush
	sio.AutoFlush(false)

	// Write something
	sio.WriteString("250 OK\r\n")

	// Buffer should not be flushed yet
	if conn.writer.Len() != 0 {
		t.Errorf("expected no write before flush, got %q", conn.writer.String())
	}

	// Now read — should NOT flush automatically
	_, err := sio.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Still no flush
	if conn.writer.Len() != 0 {
		t.Errorf("auto-flush was disabled but write occurred")
	}

	// Manual flush
	sio.Flush()
	if conn.writer.String() != "250 OK\r\n" {
		t.Errorf("expected flushed data, got %q", conn.writer.String())
	}
}

func TestLoggingToggle(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := &mockLogWriter{buf: logBuf}
	conn := &mockConn{reader: strings.NewReader("RSET\r\n"), writer: &bytes.Buffer{}}
	sio := New(conn, logger, 0)

	// Disable logging
	sio.Logging(false)

	_, err := sio.ReadLine()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if logBuf.Len() > 0 {
		t.Errorf("logging disabled but log written: %q", logBuf.String())
	}

	// Re-enable
	sio.Logging(true)
	sio.WriteString("250 Reset\r\n")

	if !strings.Contains(logBuf.String(), "<= 250 Reset\r\n") {
		t.Errorf("logging re-enabled but not working")
	}
}

func TestStartTLS(t *testing.T) {
	conn := &mockConn{reader: strings.NewReader("EHLO after TLS\r\n"), writer: &bytes.Buffer{}}
	sio := New(conn, nil, 0)

	// Dummy TLS config
	cfg := &tls.Config{InsecureSkipVerify: true}

	err := sio.StartTLS(cfg)
	if err != nil {
		t.Fatalf("StartTLS failed: %v", err)
	}

	// After StartTLS, we can't easily mock TLS conn, but we can check tlsEnabled
	if !sio.tlsEnabled {
		t.Error("tlsEnabled should be true after StartTLS")
	}

	// Second call should fail
	err = sio.StartTLS(cfg)
	if err == nil || err.Error() != "duplicate call StartTLS" {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestReadByte_WriteByte(t *testing.T) {
	conn := &mockConn{
		reader: strings.NewReader("ABC"),
		writer: &bytes.Buffer{},
	}
	sio := New(conn, nil, 0)

	b1, err := sio.ReadByte()
	if err != nil || b1 != 'A' {
		t.Errorf("ReadByte 1 failed: %v, got %c", err, b1)
	}

	err = sio.WriteByte('X')
	if err != nil {
		t.Fatalf("WriteByte failed: %v", err)
	}

	sio.Flush()
	if conn.writer.String() != "X" {
		t.Errorf("WriteByte not flushed: %q", conn.writer.String())
	}
}

func TestReadLine_TooLong(t *testing.T) {
	// Создаём строку длиной 4096 байт БЕЗ \n
	longLine := strings.Repeat("A", 4096)
	conn := &mockConn{reader: strings.NewReader(longLine), writer: &bytes.Buffer{}}
	sio := New(conn, nil, 0)

	_, err := sio.ReadLine()
	if !errors.Is(err, bufio.ErrBufferFull) {
		t.Errorf("expected bufio.ErrBufferFull, got %v", err)
	}
}
