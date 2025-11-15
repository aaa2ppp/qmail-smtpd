// == reckless.go ==

//go:build !paranoid
// +build !paranoid

package box

import (
	"unsafe"
)

const paranoid = false

type bufVer struct{}

// getBuf returns the buffer pointer without any checks
func (h Handle) getBuf() *buffer {
	ef := (*eface)(unsafe.Pointer(&h.value))
	return (*buffer)(unsafe.Pointer(ef.data))
}

// invalidateHandles - stub (nothing to do in reckless mode)
func (h Handle) invalidateHandles(buf *buffer) bool { return true }
