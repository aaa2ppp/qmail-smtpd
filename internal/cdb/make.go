package cdb

import (
	"bufio"
	"errors"
	"io"
	"math"
	"unsafe"
)

const DataOffset = 2048

var (
	ErrOutOfMemory = errors.New("out of memory")
)

const hpChunkSize = 1022 // align the hpList to 8192 bytes

type hp struct {
	h uint32
	p uint32
}

type hpList struct {
	hp   [hpChunkSize]hp
	num  int
	next *hpList
}

type MakeWriter interface {
	io.Writer
	io.Seeker
}

type CDBMake struct {
	buf     [8]byte
	head    *hpList
	entries uint32
	pos     uint32
	bw      *bufio.Writer
	fd      MakeWriter
}

func Make(fd MakeWriter) (*CDBMake, error) {
	if _, err := fd.Seek(DataOffset, io.SeekStart); err != nil {
		return nil, err
	}
	bw := bufio.NewWriter(fd)
	return &CDBMake{
		pos: DataOffset,
		bw:  bw,
		fd:  fd,
	}, nil
}

func (c *CDBMake) posplus(n uint32) error {
	newpos := c.pos + n
	if newpos < n {
		return ErrOutOfMemory
	}
	c.pos = newpos
	return nil
}

func (c *CDBMake) addend(keylen, datalen uint32, h uint32) error {
	head := c.head
	if head == nil || head.num >= hpChunkSize {
		head = &hpList{}
		head.next = c.head
		c.head = head
	}
	head.hp[head.num] = hp{h: h, p: c.pos}
	head.num++
	c.entries++
	if err := c.posplus(8); err != nil {
		return err
	}
	if err := c.posplus(keylen); err != nil {
		return err
	}
	if err := c.posplus(datalen); err != nil {
		return err
	}
	return nil
}

func (c *CDBMake) addbegin(keylen, datalen int) error {
	buf := c.buf[:8]

	if keylen > 0xffffffff {
		return ErrOutOfMemory
	}
	if datalen > 0xffffffff {
		return ErrOutOfMemory
	}

	uint32_pack(buf, uint32(keylen))
	uint32_pack(buf[4:], uint32(datalen))
	c.bw.Write(buf)
	return nil
}

func (c *CDBMake) Add(key []byte, data []byte) error {
	if err := c.addbegin(len(key), len(data)); err != nil {
		return err
	}
	if _, err := c.bw.Write(key); err != nil {
		return err
	}
	if _, err := c.bw.Write(data); err != nil {
		return err
	}
	if err := c.addend(uint32(len(key)), uint32(len(data)), hash(key)); err != nil {
		return err
	}
	return nil
}

func (c *CDBMake) Finish() error {
	var (
		buf   = c.buf[:8]
		count = make([]uint32, 256)
		start = make([]uint32, 256)
		final = make([]byte, 2048)
	)

	for x := c.head; x != nil; x = x.next {
		for i := x.num - 1; i >= 0; i-- {
			count[255&x.hp[i].h]++
		}
	}

	{
		u := uint32(0)
		for i := range 256 {
			u += count[i] /* bounded by numentries, so no overflow */
			start[i] = u
		}
	}

	memsize := uint32(1)
	for i := range count {
		u := count[i] * 2
		if u > memsize {
			memsize = u
		}
	}

	{
		memsize += c.entries /* no overflow possible up to now */
		u := uint32(math.MaxUint32)
		u /= uint32(unsafe.Sizeof(hp{}))
		if memsize > u {
			return ErrOutOfMemory
		}
	}

	split := make([]hp, memsize)
	hash := split[c.entries:]

	for x := c.head; x != nil; x = x.next {
		for i := x.num - 1; i >= 0; i-- {
			u := 255 & x.hp[i].h
			start[u]--
			split[start[u]] = x.hp[i]
		}
	}

	for i := range 256 {
		len_ := count[i] * 2 /* no overflow possible */
		uint32_pack(final[8*i:], c.pos)
		uint32_pack(final[8*i+4:], len_)

		clear(hash[:len_])

		hp := split[start[i]:]
		for u := range count[i] {
			where := (hp[u].h >> 8) % len_
			for hash[where].p != 0 {
				where++
				if where == len_ {
					where = 0
				}
			}
			hash[where] = hp[u]
		}

		for u := range len_ {
			uint32_pack(buf, hash[u].h)
			uint32_pack(buf[4:], hash[u].p)
			if _, err := c.bw.Write(buf); err != nil {
				return err
			}
			if err := c.posplus(8); err != nil {
				return err
			}
		}
	}

	if err := c.bw.Flush(); err != nil {
		return err
	}

	if _, err := c.fd.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := c.fd.Write(final); err != nil {
		return err
	}

	return nil
}
