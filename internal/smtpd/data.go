package smtpd

import (
	"cmp"
	"errors"
	"strconv"
	"time"

	"qmail-smtpd/internal/logger"
	"qmail-smtpd/internal/qmail"
)

func (d *Server) straynewline(ss *session) error {
	ss.out("451 See http://pobox.com/~djb/docs/smtplf.html.\r\n")
	return cmp.Or(ss.Flush(), ErrStrayNewLine)
}

func (d *Server) acceptmessage(ss *session, qp int) error {
	when := time.Now()
	_ = ss.out("250 ok ")
	_ = ss.out(strconv.Itoa(int(when.Unix())))
	_ = ss.out(" qt ")
	_ = ss.out(strconv.Itoa(qp))
	return ss.out("\r\n")
}

func (d *Server) smtp_data(ss *session, _ string) error {
	if !ss.seenmail {
		return d.err_wantmail(ss)
	}
	if len(ss.rcptto) == 0 {
		return d.err_wantrcpt(ss)
	}
	ss.seenmail = false

	if d.cfg.Qmail == nil {
		return d.err_qqt(ss)
	}

	qqt, err := d.cfg.Qmail.Begin(ss.mailfrom, ss.rcptto, ss.env)
	if err != nil {
		logger.FromContext(ss.ctx).Error("qmail.Begin", "error", err)
		return d.err_qqt(ss)
	}
	defer qqt.Rollback()

	qp := qqt.Pid()
	if err := ss.out("354 go ahead\r\n"); err != nil {
		return err
	}
	ss.Flush()

	d.received(qqt, ss)

	_, blastErr := d.blast(qqt, ss, ss.databytes)
	if blastErr == ErrStrayNewLine {
		return d.straynewline(ss)
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

	return d.acceptmessage(ss, qp)
}
