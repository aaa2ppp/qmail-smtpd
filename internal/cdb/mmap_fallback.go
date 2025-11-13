//go:build !unix && !windows

package cdb

import (
	"fmt"
	"io"
	"os"
)

// MaxMemoryMapSize limits memory usage for large files
const MaxMemoryMapSize = 1 << 20 // 1 MiB

// mmap implements memory mapping fallback.
// This stub reads the first size bytes of the file into slice and returns it.
func mmap(fd *os.File, size int) ([]byte, error) {
	if size > MaxMemoryMapSize {
		return nil, fmt.Errorf("size is too large, the maximum size for reading into memory is %d", MaxMemoryMapSize)
	}

	if _, err := fd.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	data := make([]byte, size)
	p := 0
	for p < len(data) {
		n, err := fd.Read(data[p:])
		p += n
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func munmap(_ []byte) error {
	return nil
}
