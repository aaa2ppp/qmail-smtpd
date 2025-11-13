package cdb

import (
	"testing"
)

func TestUint32PackUnpack(t *testing.T) {
	testCases := []uint32{
		0x00000000,
		0x00000001,
		0x000000FF,
		0x0000FFFF,
		0x00FFFFFF,
		0xFFFFFFFF,
		0x12345678,
		0x87654321,
		0xABCDEF00,
		0x00CDEFAB,
	}

	for _, v := range testCases {
		// Тест little-endian: pack → unpack
		buf := make([]byte, 4)
		uint32_pack(buf, v)
		unpacked := uint32_unpack(buf)
		if unpacked != v {
			t.Errorf("little-endian pack→unpack failed for 0x%08x: got 0x%08x", v, unpacked)
		}

		// Тест big-endian: pack_big → unpack_big
		uint32_pack_big(buf, v)
		unpackedBig := uint32_unpack_big(buf)
		if unpackedBig != v {
			t.Errorf("big-endian pack→unpack failed for 0x%08x: got 0x%08x", v, unpackedBig)
		}

		// Проверим конкретные байты для little-endian
		uint32_pack(buf, v)
		expectedLE := []byte{
			byte(v),
			byte(v >> 8),
			byte(v >> 16),
			byte(v >> 24),
		}
		for i := 0; i < 4; i++ {
			if buf[i] != expectedLE[i] {
				t.Errorf("little-endian byte %d mismatch for 0x%08x: got 0x%02x, want 0x%02x", i, v, buf[i], expectedLE[i])
			}
		}

		// Проверим конкретные байты для big-endian
		uint32_pack_big(buf, v)
		expectedBE := []byte{
			byte(v >> 24),
			byte(v >> 16),
			byte(v >> 8),
			byte(v),
		}
		for i := 0; i < 4; i++ {
			if buf[i] != expectedBE[i] {
				t.Errorf("big-endian byte %d mismatch for 0x%08x: got 0x%02x, want 0x%02x", i, v, buf[i], expectedBE[i])
			}
		}
	}
}

func TestUint32Roundtrip(t *testing.T) {
	// Проверяем, что pack(unpack(pack(x))) == pack(x)
	values := []uint32{0, 1, 0x12345678, 0xFFFFFFFF, 0xABCDEF01}

	for _, v := range values {
		// Little-endian
		buf1 := make([]byte, 4)
		uint32_pack(buf1, v)

		v2 := uint32_unpack(buf1)
		buf2 := make([]byte, 4)
		uint32_pack(buf2, v2)

		if string(buf1) != string(buf2) {
			t.Errorf("little-endian roundtrip failed for 0x%08x: %v != %v", v, buf1, buf2)
		}

		// Big-endian
		uint32_pack_big(buf1, v)
		v2 = uint32_unpack_big(buf1)
		uint32_pack_big(buf2, v2)

		if string(buf1) != string(buf2) {
			t.Errorf("big-endian roundtrip failed for 0x%08x: %v != %v", v, buf1, buf2)
		}
	}
}

func TestUint32Consistency(t *testing.T) {
	// Убедимся, что little и big endian дают разные, но предсказуемые результаты
	v := uint32(0x12345678)
	bufLE := make([]byte, 4)
	bufBE := make([]byte, 4)

	uint32_pack(bufLE, v)
	uint32_pack_big(bufBE, v)

	wantLE := []byte{0x78, 0x56, 0x34, 0x12}
	wantBE := []byte{0x12, 0x34, 0x56, 0x78}

	if string(bufLE) != string(wantLE) {
		t.Errorf("little-endian expected %v, got %v", wantLE, bufLE)
	}
	if string(bufBE) != string(wantBE) {
		t.Errorf("big-endian expected %v, got %v", wantBE, bufBE)
	}

	// И наоборот: unpack должен вернуть исходное
	if uint32_unpack(wantLE) != v {
		t.Errorf("little-endian unpack failed")
	}
	if uint32_unpack_big(wantBE) != v {
		t.Errorf("big-endian unpack failed")
	}
}

func TestUint32PanicOnShortSlice(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic on short slice for uint32_unpack")
		}
	}()
	_ = uint32_unpack([]byte{1, 2, 3}) // только 3 байта
}

// Аналогично для big-endian
func TestUint32PanicOnShortSliceBig(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic on short slice for uint32_unpack_big")
		}
	}()
	_ = uint32_unpack_big([]byte{1, 2, 3})
}

func TestUint32PackPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic on short slice for uint32_pack")
		}
	}()
	uint32_pack([]byte{1, 2, 3}, 0x12345678)
}

func TestUint32PackBigPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic on short slice for uint32_pack_big")
		}
	}()
	uint32_pack_big([]byte{1, 2, 3}, 0x12345678)
}
