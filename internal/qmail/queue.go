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

const qmailQueueCmd = "bin/qmail-queue"

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
	envelope  *os.File      // envelope (from/to) output
	w         *bufio.Writer // buffered message output
	stickyErr error
}

func (qt *Queue) Err() error {
	return qt.stickyErr
}

func (qt *Queue) Pid() int {
	return qt.queue.Process.Pid
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
// The transaction must be committed with Commit() or aborted with Rollback().
//
// After Commit or Rollback, all operations return ErrTxDone or any errors.
//
// On I/O error, the transaction fails immediately, and subsequent operations
// return a *TemporaryError.
//
// On non-zero exit code from qmail-queue, the error is mapped to *PermanentError
// or *TemporaryError using the original qmail logic.
func Begin(mailFrom string, rcptTo []string, env Env) (qt *Queue, err error) {
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

	binpath := cmp.Or(env.QmailQueue, qmailQueueCmd)
	queue = exec.Command(binpath)

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
	queue.Env = env.Prepare()

	if err := queue.Start(); err != nil {
		return nil, err
	}

	return &Queue{
		mailFrom: mailFrom,
		rcptTo:   rcptTo,
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

	// прежде чем подписывать конверт, мы *должны* закрыть сообщение
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

	buf.WriteByte('F')
	buf.WriteString(qt.mailFrom)
	buf.WriteByte(0)

	for _, to := range qt.rcptTo {
		buf.WriteByte('T')
		buf.WriteString(to)
		buf.WriteByte(0)
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

func (qt *Queue) close() error {
	return qt.closeWithError(nil)
}

func (qt *Queue) closeWithError(ioErr error) (err error) {
	defer func() {
		if err != nil {
			qt.stickyErr = err
			return
		}
		qt.stickyErr = ErrTxDone
	}()

	if qt.message != nil {
		ioErr = cmp.Or(ioErr, qt.message.Close())
		qt.message = nil
	}

	if qt.envelope != nil {
		ioErr = cmp.Or(ioErr, qt.envelope.Close())
		qt.envelope = nil
	}

	waitErr := qt.queue.Wait()
	exitCode := qt.queue.ProcessState.ExitCode()

	// Проверяем, завершился ли процесс сигналом
	if exitCode == -1 {
		return &TemporaryError{"qq crashed or terminated by signal (#4.3.0)"}
	}

	// Проверяем код выхода qmail-queue (должен пропустиь только exitCode == 0)
	if err := queueError(exitCode); err != nil {
		return err
	}

	// В нормальных условиях — не должно быть (exitCode == 0 и waitErr != nil)
	if waitErr != nil {
		return &TemporaryError{"qq wait err surprise (#4.3.0)"}
	}

	// Ошибки ввода-вывода (pipe broken etc). Мы сами ее поймали, но queue-queue счтитает, что все ok
	if ioErr != nil {
		return &TemporaryError{"qq read error (#4.3.0)"}
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
