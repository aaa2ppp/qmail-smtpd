// base64.go
//
// Safe wrappers around encoding/base64 that work with strings only.
// Uses unsafe to avoid allocations, but guarantees no exposure of mutable buffers.
// All functions return string or bool — never []byte.
//
// Rationale: in SMTP auth logic, we work with credentials and tokens.
// Using []byte increases risk of accidental mutation, reuse, or leakage.
// Strings are immutable in Go — this is a feature we rely on.
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
