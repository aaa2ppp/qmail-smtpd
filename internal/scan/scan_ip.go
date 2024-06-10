package scan

// TODO: не на месте определение этой структуры
type IPAddress struct {
	d [4]byte
}

func ScanIP(s string) (int, IPAddress) {
	var l int
	var ip IPAddress

	for octet := 0; octet < 4; octet++ {
		if octet > 0 {
			if len(s) == 0 || s[0] != '.' {
				return 0, ip
			}
			s = s[1:]
			l++
		}

		i, u := ScanUlong(s)
		if i == 0 {
			return 0, ip
		}
		ip.d[octet] = byte(u)
		s = s[i:]
		l += i
	}

	return l, ip
}

func ScanIPBracket(s string) (int, IPAddress) {
	var l int
	var ip IPAddress

	if len(s) == 0 || s[0] != '[' {
		return 0, ip
	}
	l, ip = ScanIP(s[1:])
	if l == 0 {
		return 0, ip
	}
	if l+1 >= len(s) || s[l+1] != ']' {
		return 0, ip
	}

	return l + 2, ip
}
