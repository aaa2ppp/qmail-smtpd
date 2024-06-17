package log

import (
	"io"
	"unsafe"
)

type Writer struct {
	Out      io.Writer
	Prefix   string
	buf      []byte
	notFirst bool // not first char in line
}

func (l *Writer) Write(b []byte) (int, error) {
	for _, c := range b {
		l.WriteByte(c)
	}
	return len(b), nil
}

func (l *Writer) WriteByte(c byte) error {
	if !l.notFirst { // it first char in line
		l.buf = append(l.buf, l.Prefix...)
	}
	if c == '\r' {
		return nil
	}
	l.buf = append(l.buf, c)
	l.notFirst = c != '\n'
	return nil
}

func (l *Writer) WriteString(s string) (int, error) {
	return l.Write(unsafe.Slice(unsafe.StringData(s), len(s)))
}

func (l *Writer) Flush() error {
	l.Out.Write(l.buf)
	l.buf = l.buf[:0]
	return nil // XXX ignore all errors
}
