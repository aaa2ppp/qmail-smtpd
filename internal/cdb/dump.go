package cdb

import (
	"bufio"
	"io"
	"iter"
	"math"
)

type DumpItem struct {
	Key  []byte
	Data []byte
	Err  error
}

// Dump итерируется по всем записям CDB базы данных.
//
// ВАЖНО: данные в DumpItem.Key и DumpItem.Data действительны только до следующей итерации!
// Они используют внутренний буфер чтения и будут перезаписаны при следующем вызове.
// Если нужно сохранить ключ или данные между итерациями - скопируйте их.
func Dump(r io.Reader) (iter.Seq[DumpItem], error) {
	br := bufio.NewReader(r)

	dataEnd := uint32(math.MaxUint32)

	header, err := br.Peek(2048)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 256; i++ {
		hpos := uint32_unpack(header[i*8:])
		dataEnd = min(dataEnd, hpos)
	}
	br.Discard(2048)

	return func(yield func(DumpItem) bool) {
		pos := uint32(2048)
		for pos < dataEnd {
			recHeader, err := br.Peek(8)
			if err != nil {
				yield(DumpItem{Err: err})
				return
			}
			keylen := uint32_unpack(recHeader)
			datalen := uint32_unpack(recHeader[4:])
			br.Discard(8)
			pos += 8

			if keylen > math.MaxUint32-datalen {
				yield(DumpItem{Err: ErrInvalidFormat})
				return
			}

			recSize := keylen + datalen
			record, err := br.Peek(int(recSize))
			if err != nil {
				yield(DumpItem{Err: err})
				return
			}

			key := record[:keylen]
			data := record[keylen : keylen+datalen]
			if !yield(DumpItem{Key: key, Data: data}) {
				return
			}
			br.Discard(int(recSize))
			pos += recSize
		}
	}, nil
}
