package main

type ip_address struct {
	d [4]byte
}

func ip_scan(s string) (int, ip_address) {
	var l int
	var ip ip_address

	for octet := 0; octet < 4; octet++ {
		if octet > 0 {
			if len(s) == 0 || s[0] != '.' {
				return 0, ip
			}
			s = s[1:]
			l++
		}

		i, u := scan_ulong(s)
		if i == 0 {
			return 0, ip
		}
		ip.d[octet] = byte(u)
		s = s[i:]
		l += i
	}

	return l, ip
}

func ip_scanbracket(s string) (int, ip_address) {
	var l int
	var ip ip_address

	if len(s) == 0 || s[0] != '[' {
		return 0, ip
	}
	l, ip = ip_scan(s[1:])
	if l == 0 {
		return 0, ip
	}
	if l+1 > len(s) || s[l+1] != ']' {
		return 0, ip
	}

	return l + 2, ip
}
