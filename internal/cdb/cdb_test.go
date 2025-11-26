package cdb_test

import (
	"errors"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"qmail-smtpd/internal/cdb"
)

func TestCDBCreateAndRead(t *testing.T) {
	// Создаем временный файл
	tmpfile, err := os.CreateTemp("", "cdb_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	// Создаем CDB
	maker, err := cdb.Make(tmpfile)
	if err != nil {
		t.Fatal(err)
	}

	// Добавляем тестовые данные
	testData := []struct {
		key  string
		data string
	}{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
		{"", "empty key"}, // пустой ключ
		{"empty", ""},     // пустое значение
	}

	for _, item := range testData {
		if err := maker.Add([]byte(item.key), []byte(item.data)); err != nil {
			t.Fatalf("Failed to add %s: %v", item.key, err)
		}
	}

	// Завершаем создание
	if err := maker.Finish(); err != nil {
		t.Fatal(err)
	}

	// Открываем для чтения
	cdbFile, err := cdb.OpenFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cdbFile.Close()

	// Проверяем чтение данных
	cdbFinder := cdb.NewQuery(cdbFile)
	for _, item := range testData {
		err := cdbFinder.Find(item.key)
		if err != nil {
			t.Errorf("Failed to find key %s: %v", item.key, err)
			continue
		}

		// Читаем данные
		data, err := cdbFinder.ReadBytes()
		if err != nil {
			t.Errorf("Failed to read data for key %s: %v", item.key, err)
			continue
		}

		if string(data) != item.data {
			t.Errorf("Data mismatch for key %s: got %s, want %s", item.key, string(data), item.data)
		}
	}
}

func TestCDBMultipleValues(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "cdb_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	// Создаем CDB с несколькими значениями
	maker, err := cdb.Make(tmpfile)
	if err != nil {
		t.Fatal(err)
	}

	// Добавляем несколько пар ключ-значение
	data := map[string][]string{
		"user1": {"alice", "alice@example.com", "admin"},
		"user2": {"bob", "bob@example.com", "user"},
		"user3": {"charlie", "charlie@example.com", "moderator"},
	}

	for key, values := range data {
		for _, value := range values {
			if err := maker.Add([]byte(key), []byte(value)); err != nil {
				t.Fatalf("Failed to add %s: %v", key, err)
			}
		}
	}

	if err := maker.Finish(); err != nil {
		t.Fatal(err)
	}

	// Проверяем чтение
	cdbFile, err := cdb.OpenFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cdbFile.Close()

	cdbFinder := cdb.NewQuery(cdbFile)
	for key, expectedValues := range data {
		foundValues := []string{}

		// Используем FindStart/FindNext для поиска всех значений
		cdbFinder.FindStart(key)
		for {
			err := cdbFinder.FindNext()
			if errors.Is(err, cdb.ErrNotFound) {
				break
			}
			if err != nil {
				t.Errorf("Error finding next for key %s: %v", key, err)
				break
			}

			data, err := cdbFinder.ReadBytes()
			if err != nil {
				t.Errorf("Failed to read data for key %s: %v", key, err)
				break
			}
			foundValues = append(foundValues, string(data))
		}

		// Проверяем, что нашли все значения
		if len(foundValues) != len(expectedValues) {
			t.Errorf("Wrong number of values for key %s: got %d, want %d",
				key, len(foundValues), len(expectedValues))
		}

		// Проверяем содержимое
		for _, expected := range expectedValues {
			found := false
			for _, actual := range foundValues {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Value %s not found for key %s", expected, key)
			}
		}
	}
}

func TestCDBNotFound(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "cdb_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	// Создаем пустой CDB
	maker, err := cdb.Make(tmpfile)
	if err != nil {
		t.Fatal(err)
	}

	// Добавляем только один ключ
	if err := maker.Add([]byte("existing"), []byte("value")); err != nil {
		t.Fatal(err)
	}

	if err := maker.Finish(); err != nil {
		t.Fatal(err)
	}

	// Проверяем поиск несуществующего ключа
	cdbFile, err := cdb.OpenFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cdbFile.Close()

	cdbFinder := cdb.NewQuery(cdbFile)

	// Ключ должен существовать
	err = cdbFinder.Find("existing")
	if err != nil {
		t.Errorf("Existing key not found: %v", err)
	}

	// Ключ не должен существовать
	err = cdbFinder.Find("nonexistent")
	if !errors.Is(err, cdb.ErrNotFound) {
		t.Errorf("Expected ErrNotFound for nonexistent key, got: %v", err)
	}
}

func TestCDBLargeData(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "cdb_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	// Создаем CDB с большими данными
	maker, err := cdb.Make(tmpfile)
	if err != nil {
		t.Fatal(err)
	}

	// Большой ключ и большое значение
	largeKey := strings.Repeat("a", 1000)
	largeValue := strings.Repeat("b", 5000)

	if err := maker.Add([]byte(largeKey), []byte(largeValue)); err != nil {
		t.Fatal(err)
	}

	if err := maker.Finish(); err != nil {
		t.Fatal(err)
	}

	// Проверяем чтение
	cdbFile, err := cdb.OpenFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cdbFile.Close()

	cdbFinder := cdb.NewQuery(cdbFile)
	err = cdbFinder.Find(largeKey)
	if err != nil {
		t.Fatalf("Failed to find large key: %v", err)
	}

	data, err := cdbFinder.ReadBytes()
	if err != nil {
		t.Fatalf("Failed to read large data: %v", err)
	}

	if string(data) != largeValue {
		t.Errorf("Large data mismatch: got %d bytes, want %d bytes",
			len(data), len(largeValue))
	}
}

func TestCDBEmptyFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "cdb_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Пытаемся открыть пустой файл как CDB
	_, err = cdb.OpenFile(tmpfile.Name())
	if err == nil {
		t.Error("Expected error opening empty file as CDB")
	}
}

func TestCDBInvalidFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "cdb_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Записываем некорректные данные
	n := 0
	for n < cdb.DataOffset {
		n2, err := tmpfile.Write([]byte("not a cdb file\n"))
		if err != nil {
			t.Fatal(err)
		}
		n += n2
	}
	tmpfile.Close()

	// Пытаемся открыть как CDB
	cdbFile, err := cdb.OpenFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to open invalid file: %v", err)
	}
	defer cdbFile.Close()

	cdbFinder := cdb.NewQuery(cdbFile)
	err = cdbFinder.Find("anykey")
	if err == nil {
		t.Error("Expected error when searching in invalid CDB file")
	}
}

func BenchmarkCDBCreate(b *testing.B) {
	tmpfile, err := os.CreateTemp("", "cdb_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	keys := generateKeys('a', 'z', 1<<20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		maker, err := cdb.Make(tmpfile)
		if err != nil {
			b.Fatal(err)
		}

		for _, key := range keys {
			value := strings.ToUpper(key)
			if err := maker.Add([]byte(key), []byte(value)); err != nil {
				b.Fatal(err)
			}
		}

		if err := maker.Finish(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCDBLookup(b *testing.B) {
	// Создаем тестовую базу
	tmpfile, err := os.CreateTemp("", "cdb_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	maker, err := cdb.Make(tmpfile)
	if err != nil {
		b.Fatal(err)
	}

	keys := generateKeys8('a', 'z', 1<<20)

	// Добавляем тестовые данные
	for _, key := range keys {
		value := strings.ToUpper(key)
		if err := maker.Add([]byte(key), []byte(value)); err != nil {
			b.Fatal(err)
		}
	}

	if err := maker.Finish(); err != nil {
		b.Fatal(err)
	}
	tmpfile.Close()

	// Бенчмарк поиска
	cdbFile, err := cdb.OpenFile(tmpfile.Name())
	if err != nil {
		b.Fatal(err)
	}
	defer cdbFile.Close()

	q := cdb.NewQuery(cdbFile)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		v, _ := q.Get(key)
		_ = v
	}
}

func generateKeys(start, end byte, n int) []string {
	keys := make([]string, 0, n)
	key := make([]byte, 0, 10)
	j := -1
	for range n {
		if j == -1 {
			key = append(key, start)
			j = len(key) - 1
		} else if key[j] == end {
			key[j] = start
			j--
			continue
		} else {
			key[j]++
			j = len(key) - 1
		}
		keys = append(keys, string(key))
	}
	rand.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	return keys
}

func generateKeys8(start, end byte, n int) []string {
	keys := make([]string, 0, n)
	key := make([]byte, 8)
	for range n {
		for i := range key {
			v := rand.IntN(int(end-start+1)) + int(start)
			key[i] = byte(v)
		}
		keys = append(keys, string(key))
	}
	return keys
}
