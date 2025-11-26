package cdb

import "testing"

func TestCDBHashFunction(t *testing.T) {
	// Проверяем консистентность хеш-функции
	testCases := []struct {
		input    string
		expected uint32
	}{
		{"", 5381},
		{"a", (5381 * 33) ^ 'a'},
		{"ab", (((5381 * 33) ^ 'a') * 33) ^ 'b'},
		{"hello", 0x0a9cede7}, // ожидаемое значение для нашей хеш-функции
	}

	for _, tc := range testCases {
		result := hashString(tc.input)
		if tc.expected != 0 && result != tc.expected {
			t.Errorf("Hash mismatch for '%s': got 0x%08x", tc.input, result)
		}

		// Проверяем, что хеш детерминистичен
		result2 := hashString(tc.input)
		if result != result2 {
			t.Errorf("Hash not deterministic for '%s': 0x%08x != 0x%08x",
				tc.input, result, result2)
		}
	}
}

func BenchmarkCDBHash(b *testing.B) {
	keys := []string{
		"short",
		"medium_length_key",
		"very_long_key_that_should_test_performance_well",
		"",
		"1234567890",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			hashString(key)
		}
	}
}

var sink uint32

func BenchmarkHash(b *testing.B) {
	b.Run("string", func(b *testing.B) {
		testString := "hello world"
		for i := 0; i < b.N; i++ {
			sink = hashString(testString)
		}
	})
	b.Run("bytes", func(b *testing.B) {
		testBytes := []byte("hello world")
		for i := 0; i < b.N; i++ {
			sink = hash(testBytes)
		}
	})
}
