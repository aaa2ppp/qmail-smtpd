package main

func scan_ulong(s string) (int, uint) {
	var pos int
	var result uint
	for pos < len(s) {
		c := uint(s[pos] - '0')
		if !( c < 10) {
			break
		}
		result = result * 10 + c
		pos++ 
	}
	return pos, result
}
