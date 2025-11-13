package cdb

const HashStart = 5381

func hashadd(h uint32, c byte) uint32 {
	h += (h << 5)
	return h ^ uint32(c)
}

func hashString(key string) uint32 {
	return hash([]byte(key))
}

func hash(key []byte) uint32 {
	h := uint32(HashStart)
	for _, c := range key {
		h = hashadd(h, c)
	}
	return h
}
