package smtpd

import (
	"errors"
)

var (
	ErrStrayNewLine      = errors.New("stray new line")
	ErrExceedingMaxHops  = errors.New("exceeding max hops")
	ErrDatabytesOverflow = errors.New("databytes overflow")
)

func (d *Server) blast(qqt Queue, ss *session, limit int) (bytesRead int, _ error) {
	var (
		hops         = 0
		state        = 1
		flaginheader = true
		pos          = 0    // number of bytes since most recent \n, if fih
		flagmaybex   = true // true if this line might match RECEIVED, if fih
		flagmaybey   = true // true if this line might match \r\n, if fih
		flagmaybez   = true // true if this line might match DELIVERED, if fih
		anyErr       error
	)

	setError := func(err error) {
		if anyErr == nil {
			anyErr = err
			qqt.Rollback()
		}
	}

	put := func(ch byte) {
		if anyErr != nil {
			return
		}
		_ = qqt.WriteByte(ch)
	}

	if limit == 0 {
		limit = -1
	}

	for {
		ch, err := ss.ReadByte()
		if err != nil {
			return bytesRead, err
		}
		if bytesRead == limit {
			// обнаружили, что сообщение больше лимита
			setError(ErrDatabytesOverflow)
		}
		bytesRead++

		if flaginheader {
			if pos < 9 {
				if ch != "delivered"[pos] && ch != "DELIVERED"[pos] {
					flagmaybez = false
				}
				if flagmaybez && pos == 8 {
					if hops == MaxHops {
						// обнаружили, что привышено число хопов
						setError(ErrExceedingMaxHops)
					}
					hops++
				}
				if pos < 8 {
					if ch != "received"[pos] && ch != "RECEIVED"[pos] {
						flagmaybex = false
					}
				}
				if flagmaybex && pos == 7 {
					if hops == MaxHops {
						// обнаружили, что привышено число хопов
						setError(ErrExceedingMaxHops)
					}
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
				return bytesRead, ErrStrayNewLine
			}
			if ch == '\r' {
				state = 4
				continue
			}
		case 1: /* \r\n */
			if ch == '\n' {
				return bytesRead, ErrStrayNewLine
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
				return bytesRead, ErrStrayNewLine
			}
			if ch == '\r' {
				state = 3
				continue
			}
			state = 0
		case 3: /* \r\n + .\r */
			if ch == '\n' {
				return bytesRead, anyErr
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
