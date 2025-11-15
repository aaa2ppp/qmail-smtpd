// == paranoid.go ==

//go:build paranoid
// +build paranoid

package box

import (
	"log"
	"sync/atomic"
	"unsafe"
)

func init() {
	log.Println("BOX: PARANOID mode enabled - panic when a violation of the ownership contract is detected")
}

const paranoid = true

// bufVer - buffer generation counter.
type bufVer = uintptr

// getBuffer returns a pointer to the buffer or nil if the buffer has been transfered, released or uninitialized.
func (h Handle) getBuf() *buffer {
	// get a pointer to the buffer data from the interface
	ef := (*eface)(unsafe.Pointer(&h.value))
	if ef.data == nil {
		return nil
	}

	// restore the pointer to buffer
	buf := (*buffer)(unsafe.Pointer(uintptr(unsafe.Pointer(ef.data)) - unsafe.Offsetof(buffer{}.data)))
	if buf.ver != h.bufVer {
		return nil
	}

	return buf
}

// invalidate all Handle copies.
func (h Handle) invalidateHandles(buf *buffer) bool {
	return atomic.CompareAndSwapUintptr(&buf.ver, h.bufVer, h.bufVer+1)
}
