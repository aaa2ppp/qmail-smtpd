package smtpd

import (
	"errors"
)

// in smtpd_data:
//
// if (databytes) bytestooverflow = databytes + 1;
// ...
// blast(&hops)
// ...
// if (databytes) if (!bytestooverflow) { out("552 sorry, that message size exceeds my databytes limit (#5.3.4)\r\n"); return; }
// ...

// void put(ch)
// char *ch;
// {
//   if (bytestooverflow)
//     if (!--bytestooverflow)
//       qmail_fail(&qqt);
//   qmail_put(&qqt,ch,1);
// }

// void blast(hops)
// int *hops;
// {
//   char ch;
//   int state;
//   int flaginheader;
//   int pos; /* number of bytes since most recent \n, if fih */
//   int flagmaybex; /* 1 if this line might match RECEIVED, if fih */
//   int flagmaybey; /* 1 if this line might match \r\n, if fih */
//   int flagmaybez; /* 1 if this line might match DELIVERED, if fih */

//   state = 1;
//   *hops = 0;
//   flaginheader = 1;
//   pos = 0; flagmaybex = flagmaybey = flagmaybez = 1;
//   for (;;) {
//     substdio_get(&ssin,&ch,1);
//     if (flaginheader) {
//       if (pos < 9) {
//         if (ch != "delivered"[pos]) if (ch != "DELIVERED"[pos]) flagmaybez = 0;
//         if (flagmaybez) if (pos == 8) ++*hops;
//         if (pos < 8)
//           if (ch != "received"[pos]) if (ch != "RECEIVED"[pos]) flagmaybex = 0;
//         if (flagmaybex) if (pos == 7) ++*hops;
//         if (pos < 2) if (ch != "\r\n"[pos]) flagmaybey = 0;
//         if (flagmaybey) if (pos == 1) flaginheader = 0;
// 	++pos;
//       }
//       if (ch == '\n') { pos = 0; flagmaybex = flagmaybey = flagmaybez = 1; }
//     }
//     switch(state) {
//       case 0:
//         if (ch == '\n') straynewline();
//         if (ch == '\r') { state = 4; continue; }
//         break;
//       case 1: /* \r\n */
//         if (ch == '\n') straynewline();
//         if (ch == '.') { state = 2; continue; }
//         if (ch == '\r') { state = 4; continue; }
//         state = 0;
//         break;
//       case 2: /* \r\n + . */
//         if (ch == '\n') straynewline();
//         if (ch == '\r') { state = 3; continue; }
//         state = 0;
//         break;
//       case 3: /* \r\n + .\r */
//         if (ch == '\n') return;
//         put(".");
//         put("\r");
//         if (ch == '\r') { state = 4; continue; }
//         state = 0;
//         break;
//       case 4: /* + \r */
//         if (ch == '\n') { state = 1; break; }
//         if (ch != '\r') { put("\r"); state = 0; }
//     }
//     put(&ch);
//   }
// }

const (
	MaxHops = 100
)

var (
	ErrStrayNewLine      = errors.New("stray new line")
	ErrExceedingMaxHops  = errors.New("exceeding max hops")
	ErrDatabytesOverflow = errors.New("databytes overflow")
)

func blast(qqt Queue, ss *session, limit int) (bytesRead int, _ error) {
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

	if limit == 0 {
		limit = -1
	}

	put := func(ch byte) {
		if bytesRead == limit {
			// обнаружили, что превышен размер сообщения
			setError(ErrDatabytesOverflow)
		}
		bytesRead++

		if anyErr == nil {
			anyErr = qqt.WriteByte(ch)
		}
	}

	for {
		ch, err := ss.io.ReadByte()
		if err != nil {
			return bytesRead, err
		}

		if flaginheader {
			if pos < 9 {
				if ch != "delivered"[pos] && ch != "DELIVERED"[pos] {
					flagmaybez = false
				}
				if flagmaybez && pos == 8 {
					if hops == MaxHops {
						// обнаружили, что превышено число хопов
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
						// обнаружили, что превышено число хопов
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
		case 1: // \r\n
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
		case 2: // \r\n + .
			if ch == '\n' {
				return bytesRead, ErrStrayNewLine
			}
			if ch == '\r' {
				state = 3
				continue
			}
			state = 0
		case 3: // \r\n + .\r
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
		case 4: // + \r
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
