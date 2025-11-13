package cdb

func uint32_unpack(s []byte) (result uint32) {
	if len(s) < 4 {
		panic("out of range")
	}
	result = uint32(s[3])
	result <<= 8
	result += uint32(s[2])
	result <<= 8
	result += uint32(s[1])
	result <<= 8
	result += uint32(s[0])
	return result
}

func uint32_unpack_big(s []byte) (result uint32) {
	if len(s) < 4 {
		panic("out of range")
	}
	result = uint32(s[0])
	result <<= 8
	result += uint32(s[1])
	result <<= 8
	result += uint32(s[2])
	result <<= 8
	result += uint32(s[3])
	return result
}

func uint32_pack(s []byte, u uint32) {
	if len(s) < 4 {
		panic("out of range")
	}
	s[0] = byte(u)
	u >>= 8
	s[1] = byte(u)
	u >>= 8
	s[2] = byte(u)
	u >>= 8
	s[3] = byte(u)
}

func uint32_pack_big(s []byte, u uint32) {
	if len(s) < 4 {
		panic("out of range")
	}
	s[3] = byte(u)
	u >>= 8
	s[2] = byte(u)
	u >>= 8
	s[1] = byte(u)
	u >>= 8
	s[0] = byte(u)
}
