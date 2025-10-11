package smtpd

import (
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/smtpd/safeio"
)

type sessionState struct {
	relayclient   string
	relayclientok bool
	helohost      string
	fakehelo      string /* pointer into helohost, or 0 */
	seenmail      bool
	flagbarf      bool /* defined if seenmail */
	mailfrom      string
	rcptto        []string
	authorized    bool
	user          string
	tlsEnabled    bool
}

type session struct {
	*safeio.SafeIO
	sessionState
	env qmail.Env
}

func (ss *session) out(s string) error {
	_, err := ss.WriteString(s)
	return err
}
