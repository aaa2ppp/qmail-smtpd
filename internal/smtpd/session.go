package smtpd

import (
	"context"
	"crypto/tls"
	"net"

	"qmail-smtpd/internal/env"
	"qmail-smtpd/internal/smtpd/safeio"
)

type AddrMatcher interface {
	Match(ctx context.Context, addr string) bool
}

type IPSet interface {
	Contains(ip net.IP) bool
}

type state struct {
	heloHost string
	fakeHelo bool // true if heloHost differs from remoteHost

	authorized  bool
	username    string
	relayClient bool
	relaySuffix string

	seenMail  bool
	flagbarf  bool // true if the mail from address is invalid
	bmfReason string
	mailFrom  string
	rcptTo    []string
}

type session struct {
	ctx   context.Context
	env   env.Env
	io    *safeio.SafeIO
	qmail Qmail
	ipme  IPSet

	proto       string
	greeting    string
	localIPHost string
	local       string
	remoteIP    string
	remoteHosts string
	remoteInfo  string
	databytes   int

	openRelay   bool
	rcptHosts   AddrMatcher
	badMailFrom AddrMatcher
	noMailbox   AddrMatcher

	tlsConfig  *tls.Config
	tlsEnabled bool // true if the STARTTLS was successful

	auth        Authenticator
	authFQDN    string
	relayClient bool
	relaySuffix string

	state state
}

func (ss *session) out(s string) error {
	_, err := ss.io.WriteString(s)
	return err
}
