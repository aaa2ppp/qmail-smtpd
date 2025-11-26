package safeio

import (
	"bytes"
	"io"
)

type logWriter struct {
	out       io.Writer
	prefix    string
	buf       []byte
	firstChar bool
}

func newLogWriter(out io.Writer, prefix string) *logWriter {
	return &logWriter{
		out:       out,
		prefix:    prefix,
		firstChar: true,
	}
}

func (w *logWriter) WriteByte(c byte) error {
	if w.firstChar {
		w.buf = append(w.buf, w.prefix...)
	}
	w.buf = append(w.buf, c)
	w.firstChar = (c == '\n')
	return nil
}

func (w *logWriter) Write(b []byte) (int, error) {
	for _, c := range b {
		w.WriteByte(c)
	}
	return len(b), nil
}

func (w *logWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *logWriter) Flush() error {
	for i := 0; i < len(w.buf); {
		j := i + bytes.IndexByte(w.buf[i:], '\n') + 1
		if j == i {
			j = len(w.buf)
		}
		w.out.Write(w.buf[i:j])
		i = j
	}
	w.buf = w.buf[:0]
	return nil
}
