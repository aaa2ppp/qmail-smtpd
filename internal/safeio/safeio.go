package safeio

import (
	"errors"
	"io"
	"time"
)

var ErrIOTimeout = errors.New("io timeout")

type Writer struct {
	wtr       io.Writer
	timeout   time.Duration
	stickyErr error
}

func NewWriter(w io.Writer, timeout time.Duration) *Writer {
	return &Writer{
		wtr:     w,
		timeout: timeout,
	}
}

func (w Writer) Write(b []byte) (int, error) {
	if w.stickyErr != nil {
		return 0, w.stickyErr
	}

	var (
		n   int
		err error
	)

	done := make(chan struct{}, 1)
	go func() {
		n, err = w.wtr.Write(b)
		done <- struct{}{}
	}()

	tm := time.NewTimer(w.timeout)
	select {
	case <-tm.C:
		w.stickyErr = ErrIOTimeout
		return 0, ErrIOTimeout
	case <-done:
		tm.Stop()
		w.stickyErr = err
		return n, err
	}
}

type Reader struct {
	rdr       io.Reader
	timeout   time.Duration
	flush     func()
	stickyErr error
}

func NewReader(r io.Reader, timeout time.Duration, flush func()) *Reader {
	return &Reader{
		rdr:     r,
		timeout: timeout,
		flush:   flush,
	}
}

func (r Reader) Read(b []byte) (int, error) {
	if r.stickyErr != nil {
		return 0, r.stickyErr
	}

	if r.flush != nil {
		r.flush()
	}

	var (
		n   int
		err error
	)

	done := make(chan struct{}, 1)
	go func() {
		n, err = r.rdr.Read(b)
		done <- struct{}{}
	}()

	tm := time.NewTimer(r.timeout)
	select {
	case <-tm.C:
		r.stickyErr = ErrIOTimeout
		return 0, ErrIOTimeout
	case <-done:
		tm.Stop()
		r.stickyErr = err
		return n, err
	}
}
