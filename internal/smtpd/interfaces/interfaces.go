package interfaces

import (
	"io"

	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/scan"
)

type AddrMatcher interface {
	Match(string) bool
}

type IPMe interface {
	Is(scan.IPAddress) bool
}

type Qmail interface {
	Begin(mailForm string, rcptTo []string, opts qmail.Env) (Queue, error)
}

type Queue interface {
	io.ByteWriter
	io.StringWriter
	Pid() int
	Rollback() error
	Commit() error
}

type LogWriter interface { // XXX
	io.StringWriter
	Flush() error
	WithPrefix(string) LogWriter
}
