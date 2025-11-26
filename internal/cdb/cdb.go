package cdb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"unsafe"
)

var (
	ErrInvalidFormat = errors.New("invalid format")
	ErrNotFound      = io.EOF // реализации, о которых я знаю, используют это значение
)

// CDB — безопасный доступ к CDB-файлу из разных горутин.
// Отвечает за локи, открытие/закрытие файла, предоставляет API для поиска.
type CDB struct {
	fileName string
	file     *File
	mu       sync.RWMutex
}

func Open(fileName string) (*CDB, error) {
	file, err := OpenFile(fileName)
	if err != nil {
		return nil, err
	}
	return &CDB{
		fileName: fileName,
		file:     file,
	}, nil
}

func (cdb *CDB) Close() error {
	cdb.mu.Lock()
	defer cdb.mu.Unlock()
	var err error
	if cdb.file != nil {
		err = cdb.file.Close()
		cdb.file = nil
	}
	return err
}

func (cdb *CDB) Reopen() error {
	cdb.mu.Lock()
	defer cdb.mu.Unlock()

	newFile, err := OpenFile(cdb.fileName)
	if err != nil {
		return err
	}

	if cdb.file != nil {
		if err := cdb.file.Close(); err != nil {
			slog.Warn("CDB.Reopen: can't close old file", "error", err)
		}
	}

	cdb.file = newFile
	return nil
}

func (cdb *CDB) Find(key string) error {
	cdb.mu.RLock()
	defer cdb.mu.RUnlock()

	if cdb.file == nil {
		return errors.New("CDB is closed")
	}

	q := &Query{r: cdb.file}
	return q.Find(key)
}

// Get - простой случай: значение по ключу
func (cdb *CDB) Get(key string) (string, error) {
	cdb.mu.RLock()
	defer cdb.mu.RUnlock()

	if cdb.file == nil {
		return "", errors.New("CDB is closed")
	}

	q := &Query{r: cdb.file}
	if err := q.Find(key); err != nil {
		return "", err
	}
	return q.ReadString()
}

// Do - универсальный способ для сложных операций
func (cdb *CDB) Do(fn func(*Query) error) error {
	cdb.mu.RLock()
	defer cdb.mu.RUnlock()

	if cdb.file == nil {
		return errors.New("CDB is closed")
	}

	q := &Query{r: cdb.file}
	return fn(q)
}

// File — обертка над *os.File. Реализует чтение через ReadAt, абстрагируя mmap или файловый дескриптор.
type File struct {
	fd   *os.File
	data fileMap
}

func OpenFile(fileName string) (*File, error) {
	fd, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	st, err := fd.Stat()
	if err != nil {
		fd.Close()
		return nil, err
	}

	size := st.Size()
	if size < DataOffset {
		fd.Close()
		return nil, ErrInvalidFormat
	}

	var data []byte
	if size <= MaxMemoryMapSize {
		x, err := mmap(fd, int(size))
		if err != nil {
			fd.Close()
			return nil, err
		}
		data = x
	}

	return &File{
		fd:   fd,
		data: fileMap(data),
	}, nil
}

func (f *File) Close() error {
	var err1, err2 error
	if f.data != nil {
		err1 = munmap(f.data)
		f.data = nil
	}
	if f.fd != nil {
		err2 = f.fd.Close()
		f.fd = nil
	}
	return errors.Join(err1, err2)
}

func (f *File) ReadAt(buf []byte, pos int64) (int, error) {
	if f.data != nil {
		return f.data.ReadAt(buf, pos)
	}
	return f.fd.ReadAt(buf, pos)
}

type fileMap []byte

func (data fileMap) ReadAt(buf []byte, pos int64) (int, error) {
	size := int64(len(data))
	if pos < 0 || pos > size {
		return 0, fmt.Errorf("%w: position out of bounds", ErrInvalidFormat)
	}
	if size-pos < int64(len(buf)) {
		return 0, fmt.Errorf("%w: not enough data", ErrInvalidFormat)
	}
	copy(buf, data[pos:])
	return len(buf), nil
}



// Query — сессия поиска и чтения данных в CDB-файле.
// Не потокобезопасна — должна использоваться в пределах одной горутины.
type Query struct {
	r      io.ReaderAt
	buf    [32]byte
	key    string
	loop   uint32 /* number of hash slots searched under this key */
	khash  uint32 /* initialized if loop is nonzero */
	kpos   uint32 /* initialized if loop is nonzero */
	hpos   uint32 /* initialized if loop is nonzero */
	hslots uint32 /* initialized if loop is nonzero */
	dpos   uint32 /* initialized if cdb_findnext() returns 1 */
	dlen   uint32 /* initialized if cdb_findnext() returns 1 */
}

func NewQuery(r io.ReaderAt) *Query {
	return &Query{r: r}
}

// FindStart настраивает новый поиск по ключу
func (q *Query) FindStart(key string) error {
	q.key = key
	q.loop = 0
	q.dpos = 0
	q.dlen = 0
	return nil
}

func (q *Query) read(buf []byte, pos uint32) error {
	_, err := q.r.ReadAt(buf, int64(pos))
	return err
}

func (q *Query) match(key string, pos uint32) (bool, error) {
	buf := q.buf[:] // dirty buffer!

	for len(key) > 0 {
		n := min(len(buf), len(key))
		if err := q.read(buf[:n], pos); err != nil {
			return false, err
		}
		if !bytes.Equal(buf[:n], []byte(key[:n])) {
			return false, nil
		}
		pos += uint32(n)
		key = key[n:]
	}

	return true, nil
}

// FindNext ищет следующие значение для ключа. Возвращает любые ошибки ввода вывода или ErrNotFound, если ключ не найден.
func (q *Query) FindNext() error {
	buf := q.buf[:8] // dirty buffer!

	q.dpos = 0
	q.dlen = 0

	if q.loop == 0 {
		u := hashString(q.key)
		if err := q.read(buf, (u&255)<<3); err != nil {
			return err
		}

		q.hslots = uint32_unpack(buf[4:])
		if q.hslots == 0 {
			return ErrNotFound
		}

		q.hpos = uint32_unpack(buf)
		q.khash = u
		u >>= 8
		u %= q.hslots
		u <<= 3
		q.kpos = q.hpos + u
	}

	for q.loop < q.hslots {
		if err := q.read(buf, q.kpos); err != nil {
			return err
		}
		pos := uint32_unpack(buf[4:])
		if pos == 0 {
			return ErrNotFound
		}
		q.loop += 1
		q.kpos += 8
		if q.kpos == q.hpos+(q.hslots<<3) {
			q.kpos = q.hpos
		}

		u := uint32_unpack(buf)
		if u == q.khash {
			if err := q.read(buf, pos); err != nil {
				return err
			}
			u = uint32_unpack(buf)
			if int(u) == len(q.key) {
				dlen := uint32_unpack(buf[4:])
				if ok, err := q.match(q.key, pos+8); err != nil {
					return err
				} else if ok {
					q.dlen = dlen
					q.dpos = pos + 8 + uint32(len(q.key))
					return nil
				}
			}
		}
	}

	return ErrNotFound
}

// Find находит первое значение для ключа. Эквивалентно последовательному вызову FindStart и FindNext.
func (q *Query) Find(key string) error {
	q.FindStart(key)
	return q.FindNext()
}

// Get находит первое значение для ключа и возвращает значение, как строку.
func (q *Query) Get(key string) (string, error) {
	_ = q.Find(key)
	return q.ReadString()
}

func (q *Query) readData() ([]byte, error) {
	if q.dpos == 0 {
		return nil, ErrNotFound
	}
	if q.dlen == 0 {
		return nil, nil
	}
	buf := make([]byte, q.dlen)
	if err := q.read(buf, q.dpos); err != nil {
		return nil, err
	}
	return buf, nil
}

// ReadBytes если ключ успешно найден предварительным вызовом FindNext, возвращает значение ключа, как слайс байт.
// Иначе возвращает ErrNotFound.
func (q *Query) ReadBytes() ([]byte, error) {
	return q.readData()
}

// ReadBytes если ключ успешно найден предварительным вызовом FindNext, возвращает значение ключа, как строку.
// Иначе возвращает ErrNotFound.
func (q *Query) ReadString() (string, error) {
	buf, err := q.readData()
	if err != nil {
		return "", err
	}
	if buf == nil {
		return "", nil
	}
	return unsafe.String(unsafe.SliceData(buf), len(buf)), nil
}
