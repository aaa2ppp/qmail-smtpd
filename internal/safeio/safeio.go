package safeio

import (
	"errors"
	"io"
	"time"
)

var ErrIOTimeout = errors.New("io timeout")

type Writer struct {
	ioOperation
}

func NewWriter(w io.Writer, timeout time.Duration) *Writer {
	return &Writer{ioOperation{fun: w.Write, timeout: timeout}}
}

func (w *Writer) Write(b []byte) (int, error) {
	return w.do(b)
}

type Reader struct {
	ioOperation
	flush func()
}

func NewReader(r io.Reader, timeout time.Duration, flush func()) *Reader {
	return &Reader{ioOperation{fun: r.Read, timeout: timeout}, flush}
}

func (r *Reader) Read(b []byte) (int, error) {
	if r.flush != nil {
		r.flush()
	}
	return r.do(b)
}

type ioOperation struct {
	fun       func([]byte) (int, error)
	timeout   time.Duration
	tm        *time.Timer
	stickyErr error
}

func (op *ioOperation) Timeout(t time.Duration) {
	op.timeout = t
}

func (op *ioOperation) do(b []byte) (int, error) {
	if op.stickyErr != nil {
		return 0, op.stickyErr
	}

	if op.timeout <= 0 {
		return op.fun(b)
	}

	type result struct {
		n   int
		err error
	}

	done := make(chan result, 1)
	go func() {
		n, err := op.fun(b)
		done <- result{n, err}
	}()

	if op.tm == nil {
		op.tm = time.NewTimer(op.timeout)
	} else {
		op.tm.Reset(op.timeout)
	}

	select {
	case <-op.tm.C:
		op.stickyErr = ErrIOTimeout
		return 0, ErrIOTimeout
	case r := <-done:
		if !op.tm.Stop() {
			<-op.tm.C
		}
		op.stickyErr = r.err
		return r.n, r.err
	}
}
