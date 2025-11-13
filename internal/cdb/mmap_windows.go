//go:build windows

package cdb

import (
	"math"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

const MaxMemoryMapSize = math.MaxUint32

func mmap(fd *os.File, size int) ([]byte, error) {
	handle, err := windows.CreateFileMapping(
		windows.Handle(fd.Fd()), nil,
		windows.PAGE_READONLY, 0, uint32(size), nil,
	)
	if err != nil {
		return nil, err
	}

	addr, err := windows.MapViewOfFile(handle, windows.FILE_MAP_READ, 0, 0, uintptr(size))
	windows.CloseHandle(handle)
	if err != nil {
		return nil, err
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	return data, nil
}

func munmap(data []byte) error {
	if data != nil {
		addr := uintptr(unsafe.Pointer(unsafe.SliceData(data)))
		return windows.UnmapViewOfFile(addr)
	}
	return nil
}
