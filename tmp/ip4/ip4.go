package ip4

import (
	"errors"
	"strconv"
	"strings"
)

func parseCIDR(cidr string) (addr, mask uint32, err error) {
	mask = ^uint32(0)
	i := strings.IndexByte(cidr, '/')
	if i != -1 {
		bits, err := strconv.Atoi(cidr[i+1:])
		if err != nil {
			return 0, 0, err
		}
		if !(0 <= bits && bits <= 32) {
			return 0, 0, errors.New("parseCIDR: bits be 0..32")
		}
		cidr = cidr[:i]
		mask <<= 32 - bits
	}

	oct := strings.Split(cidr, ".")
	if len(oct) != 4 {
		return 0, 0, errors.New("parseCIDR: must be 4 octets")
	}

	for i := 0; i < 4; i++ {
		a, err := strconv.Atoi(oct[i])
		if err != nil {
			return 0, 0, err
		}
		if !(0 <= a && a <= 255) {
			return 0, 0, errors.New("parseCIDR: octet must be 0..255")
		}
		addr |= uint32(a) << (24 - i*8)
	}

	return addr, mask, nil
}

func IsAllowed(rules [][2]string, ip string) bool {
	addr, _, err := parseCIDR(ip)
	if err != nil {
		panic(err) // TODO
	}

	for i := range rules {
		ruleAddr, ruleMask, err := parseCIDR(rules[i][0])
		if err != nil {
			panic(err) // TODO
		}
		if addr&ruleMask == ruleAddr&ruleMask {
			switch rules[i][1] {
			case "ALLOW":
				return true
			case "DENY":
				return false
			default:
				panic("unknown action") // TODO
			}
		}
	}

	// deny by default
	return false
}
