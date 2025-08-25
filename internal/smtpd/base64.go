package smtpd

import (
	"encoding/base64"
	"unsafe"
)

func b64decode(s string) (string, bool) {
	enc := base64.StdEncoding

	src := unsafe.Slice(unsafe.StringData(s), len(s))
	dst := make([]byte, enc.DecodedLen(len(s)))

	n, err := enc.Decode(dst, src)
	if err != nil {
		return "", false
	}

	return unsafe.String(unsafe.SliceData(dst), n), true
}

func b64encode(s string) string {
	enc := base64.StdEncoding

	src := unsafe.Slice(unsafe.StringData(s), len(s))
	dst := make([]byte, enc.EncodedLen(len(s)))

	enc.Encode(dst, src)

	return unsafe.String(unsafe.SliceData(dst), len(dst))
}
