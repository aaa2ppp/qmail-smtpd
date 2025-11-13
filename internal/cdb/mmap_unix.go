//go:build unix

package cdb

import (
	"math"
	"os"

	"golang.org/x/sys/unix"
)

const MaxMemoryMapSize = math.MaxUint32

func mmap(fd *os.File, size int) ([]byte, error) {
	return unix.Mmap(int(fd.Fd()), 0, size, unix.PROT_READ, unix.MAP_PRIVATE)
}

func munmap(data []byte) error {
	if data != nil {
		return unix.Munmap(data)
	}
	return nil
}
