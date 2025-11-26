package smtpd

import (
	"cmp"
	"errors"
	"strconv"
	"time"

	"qmail-smtpd/internal/logger"
	"qmail-smtpd/internal/qmail"
)

func strayNewLine(ss *session) error {
	ss.out("451 See http://pobox.com/~djb/docs/smtplf.html.\r\n")
	return cmp.Or(ss.io.Flush(), ErrStrayNewLine)
}

func acceptMessage(ss *session, qp int) error {
	when := time.Now()
	_ = ss.out("250 ok ")
	_ = ss.out(strconv.Itoa(int(when.Unix())))
	_ = ss.out(" qt ")
	_ = ss.out(strconv.Itoa(qp))
	return ss.out("\r\n")
}

func smtp_data(ss *session, _ string) error {
	if !ss.state.seenMail {
		return err_wantmail(ss)
	}
	if len(ss.state.rcptTo) == 0 {
		return err_wantrcpt(ss)
	}
	ss.state.seenMail = false

	if ss.qmail == nil {
		return err_qqt(ss)
	}

	qqt, err := ss.qmail.Begin(ss.state.mailFrom, ss.state.rcptTo, ss.env)
	if err != nil {
		logger.FromContext(ss.ctx).Error("qmail.Begin", "error", err)
		return err_qqt(ss)
	}
	defer qqt.Rollback()

	qp := qqt.Pid()
	if err := ss.out("354 go ahead\r\n"); err != nil {
		return err
	}
	ss.io.Flush()

	received(qqt, ss)

	_, blastErr := blast(qqt, ss, ss.databytes)
	if blastErr == ErrStrayNewLine {
		return strayNewLine(ss)
	}
	// TODO: log received data bytes

	if err := qqt.Commit(); err != nil {
		var (
			tempErr *qmail.TemporaryError
			permErr *qmail.PermanentError
		)
		switch {
		case errors.Is(err, qmail.ErrTxDone):
			// blast called rollback
		case errors.As(err, &tempErr):
			_ = ss.out("451 ")
			_ = ss.out(err.Error())
			return ss.out("\r\n")
		case errors.As(err, &permErr):
			_ = ss.out("554 ")
			_ = ss.out(err.Error())
			return ss.out("\r\n")
		default:
			return err
		}
	}

	if err := blastErr; err != nil {
		switch err {
		case ErrExceedingMaxHops:
			return ss.out("554 too many hops, this message is looping (#5.4.6)\r\n")
		case ErrDatabytesOverflow:
			return ss.out("552 sorry, that message size exceeds my databytes limit (#5.3.4)\r\n")
		}
		return err
	}

	return acceptMessage(ss, qp)
}
