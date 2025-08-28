package interfaces

import (
	"io"
	
	"qmail-smtpd/internal/scan"
)

type AddrMatcher interface {
	Match(string) bool
}

type IPMe interface {
	Is(scan.IPAddress) bool
}

type Qmail interface {
	Open(env []string) (QmailQueue, error)
}

type QmailQueue interface {
	Pid() int
	Putc(byte)
	Puts(string)
	From(string)
	To(string)
	Fail()
	Close() string
}

type LogWriter interface { // XXX
	io.StringWriter
	Flush() error
	WithPrefix(string) LogWriter
}
