package smtpd

import (
	"qmail-smtpd/internal/scan"
	"strings"
)

func addrparse(arg string) (string, bool) {
	terminator := '>'

	i := strings.IndexByte(arg, '<') + 1
	if i != 0 {
		arg = arg[i:]
	} else { /* partner should go read rfc 821 */
		terminator = ' '
		i = strings.IndexByte(arg, ':') + 1
		if i == 0 {
			return "", false
		}
		arg = arg[i:]
		for len(arg) > 0 && arg[0] == ' ' {
			arg = arg[1:]
		}
	}

	/* strip source route */
	if len(arg) > 0 && arg[0] == '@' {
		i = strings.IndexByte(arg, ':') + 1
		if i == 0 {
			return "", false
		}
		arg = arg[i:]
	}

	var addr strings.Builder
	var flagesc bool
	var flagquoted bool

	for _, ch := range []byte(arg) { /* copy arg to addr, stripping quotes */
		if flagesc {
			addr.WriteByte(ch)
			flagesc = false
		} else {
			if !flagquoted && ch == byte(terminator) {
				break
			}
			switch ch {
			case '\\':
				flagesc = true
			case '"':
				flagquoted = !flagquoted
			default:
				addr.WriteByte(ch)
			}
		}
	}
	/* could check for termination failure here, but why bother? */

	if addr.Len() > 900 {
		return "", false
	}
	return addr.String(), true
}

func replaceLocalIP(addr, host string, ipme IPMe) string {
	i := strings.LastIndexByte(addr, '@') + 1
	if i != 0 && i < len(addr) && addr[i] == '[' {
		l, ip := scan.ScanIPBracket(addr[i:])
		if i+l == len(addr) && ipme.Is(ip) {
			addr = addr[:i] + host
		}
	}
	return addr
}
