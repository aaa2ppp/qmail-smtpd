package smtpd

import (
	"encoding/base64"
	"unsafe"
)

func b64decode(s string) (string, bool) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", false
	}
	return unsafe.String(unsafe.SliceData(b), len(b)), true
}

func b64encode(s string) string {
	return base64.StdEncoding.EncodeToString(
		unsafe.Slice(unsafe.StringData(s), len(s)),
	)
}
