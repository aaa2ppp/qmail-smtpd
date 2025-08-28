package smtpd

import (
	"cmp"
	"errors"
)

/*
// TODO: развязать с Smtpd?
func (d *Server) put(ss *session, ch byte) {
	if ss.bytestooverflow != 0 {
		ss.bytestooverflow--
		if ss.bytestooverflow == 0 {
			ss.qqt.Fail()
		}
	}
	ss.qqt.Putc(ch)
}
*/

var ErrStrayNewLine = errors.New("stray new line")

func (d *Server) straynewline(ss *session) error {
	ss.out("451 See http://pobox.com/~djb/docs/smtplf.html.\r\n")
	return cmp.Or(ss.Flush(), ErrStrayNewLine)
}

func (d *Server) blast(ss *session, limit int) (hops int, overflow int, _ error) {
	var (
		state        = 1
		flaginheader = true
		pos          = 0    // number of bytes since most recent \n, if fih
		flagmaybex   = true // 1 if this line might match RECEIVED, if fih
		flagmaybey   = true // 1 if this line might match \r\n, if fih
		flagmaybez   = true // 1 if this line might match DELIVERED, if fih
	)

	if limit > 0 {
		overflow = limit + 1
	}

	put := func(ch byte) {
		if overflow != 0 {
			overflow--
			if overflow == 0 {
				ss.qqt.Fail()
			}
		}
		ss.qqt.Putc(ch)
	}

	for {
		ch, err := ss.ReadByte()
		if err != nil {
			return hops, overflow, err
		}

		if flaginheader {
			if pos < 9 {
				if ch != "delivered"[pos] && ch != "DELIVERED"[pos] {
					flagmaybez = false
				}
				if flagmaybez && pos == 8 {
					hops++
				}
				if pos < 8 {
					if ch != "received"[pos] && ch != "RECEIVED"[pos] {
						flagmaybex = false
					}
				}
				if flagmaybex && pos == 7 {
					hops++
				}
				if pos < 2 && ch != "\r\n"[pos] {
					flagmaybey = false
				}
				if flagmaybey && pos == 1 {
					flaginheader = false
				}
			}
			pos++
			if ch == '\n' {
				pos = 0
				flagmaybex = true
				flagmaybey = true
				flagmaybez = true
			}
		}

		switch state {
		case 0:
			if ch == '\n' {
				return hops, overflow, d.straynewline(ss)
			}
			if ch == '\r' {
				state = 4
				continue
			}
		case 1: /* \r\n */
			if ch == '\n' {
				return hops, overflow, d.straynewline(ss)
			}
			if ch == '.' {
				state = 2
				continue
			}
			if ch == '\r' {
				state = 4
				continue
			}
			state = 0
		case 2: /* \r\n + . */
			if ch == '\n' {
				return hops, overflow, d.straynewline(ss)
			}
			if ch == '\r' {
				state = 3
				continue
			}
			state = 0
		case 3: /* \r\n + .\r */
			if ch == '\n' {
				return hops, overflow, nil
			}
			put('.')
			put('\r')
			if ch == '\r' {
				state = 4
				continue
			}
			state = 0
		case 4: /* + \r */
			if ch == '\n' {
				state = 1
				break
			}
			if ch != '\r' {
				put('\r')
				state = 0
			}
		}

		put(ch)
	}
}
