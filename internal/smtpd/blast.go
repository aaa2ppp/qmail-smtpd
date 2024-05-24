package smtpd

import "qmail-smtpd/internal/safeio"

// TODO: развязать с Smtpd?

func (d *Smtpd) put(ch byte) {
	if d.bytestooverflow != 0 {
		d.bytestooverflow--
		if d.bytestooverflow == 0 {
			d.qqt.Fail()
		}
	}
	d.qqt.Putc(ch)
}

func (d *Smtpd) blast() int {
	hops := 0
	state := 1
	flaginheader := true
	pos := 0           /* number of bytes since most recent \n, if fih */
	flagmaybex := true /* 1 if this line might match RECEIVED, if fih */
	flagmaybey := true /* 1 if this line might match \r\n, if fih */
	flagmaybez := true /* 1 if this line might match DELIVERED, if fih */

	for {
		ch, err := d.ssin.ReadByte()
		if err != nil {
			if err == safeio.ErrIOTimeout {
				d.die_alarm()
			}
			d.die_read()
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
				d.straynewline()
			}
			if ch == '\r' {
				state = 4
				continue
			}
		case 1: /* \r\n */
			if ch == '\n' {
				d.straynewline()
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
				d.straynewline()
			}
			if ch == '\r' {
				state = 3
				continue
			}
			state = 0
		case 3: /* \r\n + .\r */
			if ch == '\n' {
				return hops
			}
			d.put('.')
			d.put('\r')
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
				d.put('\r')
				state = 0
			}
		}

		d.put(ch)
	}
}
