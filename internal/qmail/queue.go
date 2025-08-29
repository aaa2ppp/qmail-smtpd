package qmail

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"io"
	"os"
	"os/exec"
)

var queueCmdArgs = []string{"bin/qmail-queue"}

var ErrTxDone = errors.New("transaction has already been committed or rolled back")

type PermanentError struct{ Msg string }

func (e *PermanentError) Error() string { return e.Msg }

type TemporaryError struct{ Msg string }

func (e *TemporaryError) Error() string { return e.Msg }

type Queue struct {
	mailFrom  string
	rcptTo    []string
	queue     *exec.Cmd
	message   *os.File      // message output
	envelope  *os.File      // from/to output
	w         *bufio.Writer // buffered message output
	stickyErr error
}

func (qt *Queue) Err() error {
	return qt.stickyErr
}

func (qt *Queue) WriteByte(c byte) error {
	if qt.stickyErr != nil {
		return qt.stickyErr
	}
	if err := qt.w.WriteByte(c); err != nil {
		return qt.closeWithError(err)
	}
	return nil
}

func (qt *Queue) WriteString(s string) (int, error) {
	if qt.stickyErr != nil {
		return 0, qt.stickyErr
	}
	n, err := qt.w.WriteString(s)
	if err != nil {
		return n, qt.closeWithError(err)
	}
	return n, nil
}

// Begin starts a new transaction for sending a message.
// The transaction must be committed or rolled back.
//
// After Commit or Rollback, all operations will return ErrTxDone.
// If any I/O error occurs (e.g., write to pipe fails), the transaction is
// automatically rolled back on the first error, and all subsequent operations
// will return that error (stickyErr).
func Begin(mailFrom string, rcptTo []string, opts QmailEnv) (qt *Queue, err error) {
	var (
		queue    *exec.Cmd
		message  *os.File
		envelope *os.File
	)

	defer func() {
		if err != nil {
			if message != nil {
				message.Close()
			}
			if envelope != nil {
				envelope.Close()
			}
		}
	}()

	queue = exec.Command(queueCmdArgs[0], queueCmdArgs[1:]...)

	// message pipe
	mr, mw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer mr.Close()
	queue.Stdin, message = mr, mw

	// envelope pipe
	er, ew, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer er.Close()
	queue.Stdout, envelope = er, ew // yes, qmail-queue reads from fd=1 (stdout)

	queue.Stderr = os.Stderr
	queue.Env = prepareEnv(opts)

	if err := queue.Start(); err != nil {
		return nil, err
	}

	return &Queue{
		queue:    queue,
		message:  message,
		envelope: envelope,
		w:        bufio.NewWriter(message),
	}, nil
}

func (qt *Queue) Rollback() error {
	if qt.stickyErr != nil {
		return qt.stickyErr
	}
	return qt.close()
}

func (qt *Queue) Commit() (err error) {
	if qt.stickyErr != nil {
		return qt.stickyErr
	}

	if err := qt.flushAndCloseMessage(); err != nil {
		return qt.closeWithError(err)
	}

	if err := qt.writeAndCloseEnvelope(); err != nil {
		return qt.closeWithError(err)
	}

	return qt.close()
}

func (qt *Queue) flushAndCloseMessage() error {
	if err := qt.w.Flush(); err != nil {
		return err
	}
	if err := qt.message.Close(); err != nil {
		return err
	}
	qt.message = nil
	return nil
}

func (qt *Queue) writeAndCloseEnvelope() error {
	var buf bytes.Buffer

	envelopeWrite(&buf, 'F', qt.mailFrom)

	for _, to := range qt.rcptTo {
		envelopeWrite(&buf, 'T', to)
	}

	// в оригинальном qmail.c перед закрытием пишет еще один 0
	buf.WriteByte(0)

	if _, err := buf.WriteTo(qt.envelope); err != nil {
		return err
	}

	if err := qt.envelope.Close(); err != nil {
		return err
	}

	qt.envelope = nil
	return nil
}

func envelopeWrite(w interface {
	io.ByteWriter
	io.StringWriter
}, c byte, s string) error {
	w.WriteByte(c)
	w.WriteString(s)
	return w.WriteByte(0)
}

func (qt *Queue) close() error {
	return qt.closeWithError(nil)
}

func (qt *Queue) closeWithError(anyErr error) (err error) {
	// гарантируем stickyErr
	defer func() {
		if err != nil {
			qt.stickyErr = err
			return
		}
		qt.stickyErr = ErrTxDone
	}()

	// гарантируем, что message is closed
	if qt.message != nil {
		anyErr = cmp.Or(anyErr, qt.message.Close())
		qt.message = nil
	}

	// гарантируем, что envelope is closed
	if qt.envelope != nil {
		anyErr = cmp.Or(anyErr, qt.envelope.Close())
		qt.envelope = nil
	}

	// TODO: я это правильно интерпретировал?
	// if (wait_pid(&wstat,qq->pid) != qq->pid)
	// 	return "Zqq waitpid surprise (#4.3.0)"; // WTF?
	// if (wait_crashed(wstat))
	// 	return "Zqq crashed (#4.3.0)";
	waitErr := qt.queue.Wait()

	if anyErr != nil {
		return &TemporaryError{"qq read error (#4.3.0)"}
	}

	if waitErr != nil {
		code := qt.queue.ProcessState.ExitCode()
		if code == -1 {
			return &TemporaryError{"qq crashed (#4.3.0)"}
		}
		return queueError(code)
	}

	return nil
}

func queueError(code int) error {
	msg := queueMessage(code)
	if msg == "" {
		return nil
	}
	switch msg[1] {
	case 'D':
		return &PermanentError{msg[1:]}
	case 'Z':
		return &TemporaryError{msg[1:]}
	}
	return nil // stub, queueMessage always returns Z, D or empty message
}

// queueMessage оригинальная логика ответов
func queueMessage(code int) string {
	switch code {
	case 115: // compatibility
		fallthrough
	case 11:
		return "Denvelope address too long for qq (#5.1.3)"
	case 31:
		return "Dmail server permanently rejected message (#5.3.0)"
	case 51:
		return "Zqq out of memory (#4.3.0)"
	case 52:
		return "Zqq timeout (#4.3.0)"
	case 53:
		return "Zqq write error or disk full (#4.3.0)"
	case 0:
		return ""
	case 54:
		return "Zqq read error (#4.3.0)"
	case 55:
		return "Zqq unable to read configuration (#4.3.0)"
	case 56:
		return "Zqq trouble making network connection (#4.3.0)"
	case 61:
		return "Zqq trouble in home directory (#4.3.0)"
	case 63:
		fallthrough
	case 64:
		fallthrough
	case 65:
		fallthrough
	case 66:
		fallthrough
	case 62:
		return "Zqq trouble creating files in queue (#4.3.0)"
	case 71:
		return "Zmail server temporarily rejected message (#4.3.0)"
	case 72:
		return "Zconnection to mail server timed out (#4.4.1)"
	case 73:
		return "Zconnection to mail server rejected (#4.4.1)"
	case 74:
		return "Zcommunication with mail server failed (#4.4.2)"
	case 91:
		fallthrough
	case 81:
		return "Zqq internal bug (#4.3.0)"
	case 120:
		return "Zunable to exec qq (#4.3.0)"
	}
	if (code >= 11) && (code <= 40) {
		return "Dqq permanent problem (#5.3.0)"
	}
	return "Zqq temporary problem (#4.3.0)"
}

var (
	_ io.ByteWriter   = &Queue{}
	_ io.StringWriter = &Queue{}
)
