package tcprules

import (
	"bytes"
	"iter"
	"reflect"
	"strings"
	"testing"
)

func readAllResults(resultIter iter.Seq[[]byte]) []string {
	var result []string
	for value := range resultIter {
		result = append(result, string(value))
	}
	return result
}

func TestMultipleRanges(t *testing.T) {
	parser := NewParser(nil)

	tests := []struct {
		input string
		want  []string
	}{
		{
			input: "10.1-2.0-1.100",
			want: []string{
				"10.1.0.100", "10.1.1.100",
				"10.2.0.100", "10.2.1.100",
			},
		},
		{
			input: "192.168.1-2.10-11",
			want: []string{
				"192.168.1.10", "192.168.1.11",
				"192.168.2.10", "192.168.2.11",
			},
		},
		{
			input: "1.2.3.4", // без диапазонов
			want:  []string{"1.2.3.4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resultIter, err := parser.expandAddressRange([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := readAllResults(resultIter)

			if len(result) != len(tt.want) {
				t.Errorf("got %d addresses, want %d", len(result), len(tt.want))
			}

			for i, addr := range result {
				if addr != tt.want[i] {
					t.Errorf("result[%d] = %s, want %s", i, addr, tt.want[i])
				}
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	parser := NewParser(nil)

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "trailing dot",
			input: "192.168.1.",
			want:  []string{"192.168.1."},
		},
		{
			name:    "multiple trailing dots",
			input:   "10.0..",
			wantErr: true,
		},
		{
			name:    "invalid octet",
			input:   "192.168.256.1",
			wantErr: true,
		},
		{
			name:    "empty range",
			input:   "192.168..1",
			wantErr: true,
		},
		{
			name:    "too many addresses",
			input:   "1-200.1-200.1-200.1-200",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultIter, err := parser.expandAddressRange([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("expandAddressRange(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				result := readAllResults(resultIter)
				if !equalSlices(result, tt.want) {
					t.Errorf("expandAddressRange(%q) = %v, want %v", tt.input, result, tt.want)
				}
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSpecialAddresses(t *testing.T) {
	parser := NewParser(nil)

	specialCases := []struct {
		input string
		want  []string
	}{
		{
			input: "joe@127.0.0.1",
			want:  []string{"joe@127.0.0.1"},
		},
		{
			input: "=example.com",
			want:  []string{"=example.com"},
		},
		{
			input: "user@=hostname",
			want:  []string{"user@=hostname"},
		},
		{
			input: "key=value",
			want:  []string{"key=value"},
		},
		{
			input: "", // правило по умолчанию
			want:  []string{""},
		},
	}

	for _, tt := range specialCases {
		t.Run(tt.input, func(t *testing.T) {
			resultIter, err := parser.expandAddressRange([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			result := readAllResults(resultIter)
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("expandAddressRange(%q) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestParser_parseRule(t *testing.T) {
	parser := &Parser{}

	tests := []struct {
		name    string
		rule    string
		want    string
		wantPos int
		wantErr bool
	}{
		{
			name:    "deny",
			rule:    "deny",
			want:    "D\x00",
			wantPos: 4,
			wantErr: false,
		},
		{
			name:    "allow",
			rule:    "allow",
			want:    "",
			wantPos: 5,
			wantErr: false,
		},
		{
			name:    "allow with env",
			rule:    `allow,KEY="value"`,
			want:    "+KEY=value\x00",
			wantPos: 17,
			wantErr: false,
		},
		{
			name:    "allow with multiple env",
			rule:    `allow,KEY1="v1",KEY2="v2"`,
			want:    "+KEY1=v1\x00+KEY2=v2\x00",
			wantPos: 25,
			wantErr: false,
		},
		{
			name:    "empty value",
			rule:    `allow,EMPTY=""`,
			want:    "+EMPTY=\x00",
			wantPos: 14, // allow,EMPTY="" = 14 chars
			wantErr: false,
		},
		{
			name:    "delimiter in middle should fail",
			rule:    `allow,PATH=/usr/bin/`,
			wantPos: 16, // allow,PATH=/usr/ (до второго /)
			wantErr: true,
		},
		{
			name:    "unmatched delimiter should fail",
			rule:    `allow,PATH="/usr/bin/`,
			wantPos: 12,
			wantErr: true,
		},
		{
			name:    "custom delimiter with paired slashes",
			rule:    `allow,PATH=|usr/bin|`,
			want:    "+PATH=usr/bin\x00",
			wantPos: 20, // allow,PATH=usr/bin = 20 chars
			wantErr: false,
		},
		{
			name:    "invalid action",
			rule:    "reject",
			wantPos: 0,
			wantErr: true,
		},
		{
			name:    "missing comma",
			rule:    "allow KEY=value",
			wantPos: 5,
			wantErr: true,
		},
		{
			name:    "only opening quote",
			rule:    `allow,KEY="value`,
			wantPos: 11,
			wantErr: true,
		},
		{
			name:    "empty after comma",
			rule:    "allow,",
			wantPos: 6,
			wantErr: true,
		},
		{
			name:    "double comma",
			rule:    "allow,,KEY=value",
			wantPos: 6,
			wantErr: true,
		},
		{
			name:    "special chars in value",
			rule:    `allow,KEY="val=ue"`,
			want:    "+KEY=val=ue\x00",
			wantPos: 18,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			pos, err := parser.parseRule(&buf, tt.rule)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if pos != tt.wantPos {
				t.Errorf("parseRule() pos = %d, want %d", pos, tt.wantPos)
			}
			if !tt.wantErr {
				if got := buf.String(); got != tt.want {
					t.Errorf("parseRule() data = %q, want %q", got, tt.want)
				}
			}
		})
	}
}

// MockAdder для тестирования
type MockAdder struct {
	rules map[string]string
}

func NewMockAdder() *MockAdder {
	return &MockAdder{rules: make(map[string]string)}
}

func (m *MockAdder) Add(key []byte, data []byte) error {
	m.rules[string(key)] = string(data)
	return nil
}

func TestParser_ParseLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantRules map[string]string
		wantErr   bool
	}{
		{
			name:      "comment line",
			line:      "# this is a comment",
			wantRules: map[string]string{},
			wantErr:   false,
		},
		{
			name:      "empty line",
			line:      "",
			wantRules: map[string]string{},
			wantErr:   false,
		},
		{
			name: "simple deny",
			line: "127.0.0.1:deny",
			wantRules: map[string]string{
				"127.0.0.1": "D\x00",
			},
			wantErr: false,
		},
		{
			name: "simple allow",
			line: "192.168.1.1:allow",
			wantRules: map[string]string{
				"192.168.1.1": "",
			},
			wantErr: false,
		},
		{
			name: "allow with env variable",
			line: `:allow,RELAYCLIENT=""`,
			wantRules: map[string]string{
				"": "+RELAYCLIENT=\x00",
			},
			wantErr: false,
		},
		{
			name: "multiple env variables",
			line: `192.168.1.1:allow,RELAYCLIENT="",TCPLOCALHOST="example.com"`,
			wantRules: map[string]string{
				"192.168.1.1": "+RELAYCLIENT=\x00+TCPLOCALHOST=example.com\x00",
			},
			wantErr: false,
		},
		{
			name: "ip range single octet",
			line: "192.168.1.10-12:deny",
			wantRules: map[string]string{
				"192.168.1.10": "D\x00",
				"192.168.1.11": "D\x00",
				"192.168.1.12": "D\x00",
			},
			wantErr: false,
		},
		{
			name: "ip range multiple octets",
			line: "10.1-2.0-1.100:allow",
			wantRules: map[string]string{
				"10.1.0.100": "",
				"10.1.1.100": "",
				"10.2.0.100": "",
				"10.2.1.100": "",
			},
			wantErr: false,
		},
		{
			name: "special address with @",
			line: "user@127.0.0.1:deny",
			wantRules: map[string]string{
				"user@127.0.0.1": "D\x00",
			},
			wantErr: false,
		},
		{
			name: "special address with =",
			line: "=example.com:allow",
			wantRules: map[string]string{
				"=example.com": "",
			},
			wantErr: false,
		},
		{
			name: "default rule",
			line: ":deny",
			wantRules: map[string]string{
				"": "D\x00",
			},
			wantErr: false,
		},
		{
			name: "custom delimiter",
			line: `:allow,PATH=|usr/local/bin|`,
			wantRules: map[string]string{
				"": "+PATH=usr/local/bin\x00",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adder := NewMockAdder()
			parser := NewParser(adder)

			err := parser.ParseLine([]byte(tt.line))

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(adder.rules) != len(tt.wantRules) {
					t.Errorf("rules count = %d, want %d", len(adder.rules), len(tt.wantRules))
					return
				}

				for key, wantData := range tt.wantRules {
					gotData, exists := adder.rules[key]
					if !exists {
						t.Errorf("rule for key %q not found", key)
						continue
					}
					if gotData != wantData {
						t.Errorf("data for key %q = %v, want %v", key, gotData, wantData)
					}
				}
			}
		})
	}
}

func TestUnsafeMemorySafety(t *testing.T) {
	// Создаем сложное правило с диапазонами и env переменными
	originalRule := []byte(`  192.168.1-2.10-12:allow,RELAYCLIENT="",TCPLOCALHOST="example.com",PATH="/usr/bin/"  `)

	// Создаем копию для модификации
	ruleCopy := make([]byte, len(originalRule))
	copy(ruleCopy, originalRule)

	adder := NewMockAdder()
	parser := NewParser(adder)

	// Парсим правило
	err := parser.ParseLine(ruleCopy)
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	// Модифицируем исходные данные ПОСЛЕ парсинга
	// Это должно быть безопасно, т.к. парсер должен был скопировать все нужные данные
	for i := range ruleCopy {
		ruleCopy[i] = 'X'
	}

	// Проверяем что парсер сохранил правильные данные несмотря на модификацию исходного буфера
	expectedAddresses := []string{
		"192.168.1.10", "192.168.1.11", "192.168.1.12",
		"192.168.2.10", "192.168.2.11", "192.168.2.12",
	}

	expectedData := "+RELAYCLIENT=\x00+TCPLOCALHOST=example.com\x00+PATH=/usr/bin/\x00"

	// Проверяем все сгенерированные адреса
	for _, addr := range expectedAddresses {
		data, exists := adder.rules[addr]
		if !exists {
			t.Errorf("Rule for address %q not found", addr)
			continue
		}
		if data != expectedData {
			t.Errorf("Data for address %q = %q, want %q", addr, data, expectedData)
		}
	}

	// Проверяем что в adder.rules нет ссылок на модифицированный буфер
	for key, data := range adder.rules {
		// Проверяем что ключи не содержат 'X' (из модифицированного буфера)
		if strings.Contains(key, "X") {
			t.Errorf("Key %q contains corrupted data from modified buffer", key)
		}

		// Проверяем что данные не содержат 'X'
		if strings.Contains(data, "X") {
			t.Errorf("Data for key %q contains corrupted data from modified buffer: %q", key, data)
		}
	}
}

// Дополнительный тест для особо сложного случая
func TestUnsafeComplexRule(t *testing.T) {
	// Очень сложное правило с множеством диапазонов и env переменных
	originalRule := []byte(`10.1-2.0-1.100-101:deny,DB_HOST="db1.example.com",DB_PORT="5432",APP_ENV="production",MAX_CONN="100"`)

	ruleCopy := make([]byte, len(originalRule))
	copy(ruleCopy, originalRule)

	adder := NewMockAdder()
	parser := NewParser(adder)

	err := parser.ParseLine(ruleCopy)
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	// Жесткая модификация исходного буфера
	for i := range ruleCopy {
		ruleCopy[i] = byte(i % 256) // заполняем мусором
	}

	// Проверяем что все ожидаемые правила создались
	expectedCount := 2 * 2 * 2 // 10.1-2.0-1.100-101 = 8 комбинаций
	if len(adder.rules) != expectedCount {
		t.Errorf("Expected %d rules, got %d", expectedCount, len(adder.rules))
	}

	// Проверяем несколько конкретных адресов
	testAddresses := []string{
		"10.1.0.100", "10.1.0.101", "10.2.1.101",
	}

	expectedData := "D\x00"

	for _, addr := range testAddresses {
		data, exists := adder.rules[addr]
		if !exists {
			t.Errorf("Rule for address %q not found", addr)
			continue
		}
		if data != expectedData {
			t.Errorf("Data for address %q = %q, want %q", addr, data, expectedData)
		}
	}
}

// func TestUnsafeMemorySafety_SpecialAddresses(t *testing.T) {
// 	// Тестируем адреса с = и @, где используется unsafe
// 	tests := []struct {
// 		name    string
// 		rule    []byte
// 		wantKey string
// 	}{
// 		{
// 			name:    "address with @",
// 			rule:    []byte("user@host:allow"),
// 			wantKey: "user@host",
// 		},
// 		{
// 			name:    "address with =",
// 			rule:    []byte("key=value:deny"),
// 			wantKey: "key=value",
// 		},
// 		{
// 			name:    "complex address with both",
// 			rule:    []byte("user@host=value:allow"),
// 			wantKey: "user@host=value",
// 		},
// 		{
// 			name:    "empty address",
// 			rule:    []byte(":allow"),
// 			wantKey: "",
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			// Создаем копию для модификации
// 			ruleCopy := make([]byte, len(tt.rule))
// 			copy(ruleCopy, tt.rule)

// 			adder := NewMockAdder()
// 			parser := NewParser(adder)

// 			// Парсим правило
// 			err := parser.ParseLine(ruleCopy)
// 			if err != nil {
// 				t.Fatalf("ParseLine failed: %v", err)
// 			}

// 			// Жестко портим исходный буфер
// 			for i := range ruleCopy {
// 				ruleCopy[i] = 'X'
// 			}

// 			// Проверяем что ключ не испортился
// 			_, exists := adder.rules[tt.wantKey]
// 			if !exists {
// 				t.Errorf("Rule for key %q not found. All keys: %v", tt.wantKey, getKeys(adder.rules))
// 			}

// 			// Проверяем что в данных нет 'X'
// 			for key, data := range adder.rules {
// 				if strings.Contains(key, "X") {
// 					t.Errorf("Key %q contains corrupted data from modified buffer", key)
// 				}
// 				if bytes.Contains(data, []byte("X")) {
// 					t.Errorf("Data for key %q contains corrupted data: %q", key, data)
// 				}
// 			}
// 		})
// 	}
// }

// func TestUnsafeMemorySafety_RulePart(t *testing.T) {
// 	// Тестируем часть после :, где используется unsafe для rulePart
// 	rule := []byte("192.168.1.1:allow,KEY=\"value\"")

// 	ruleCopy := make([]byte, len(rule))
// 	copy(ruleCopy, rule)

// 	adder := NewMockAdder()
// 	parser := NewParser(adder)

// 	err := parser.ParseLine(ruleCopy)
// 	if err != nil {
// 		t.Fatalf("ParseLine failed: %v", err)
// 	}

// 	// Портим исходный буфер
// 	for i := range ruleCopy {
// 		ruleCopy[i] = '*'
// 	}

// 	// Проверяем что данные не испортились
// 	data, exists := adder.rules["192.168.1.1"]
// 	if !exists {
// 		t.Fatal("Rule not found")
// 	}

// 	expected := []byte("+KEY=value\x00")
// 	if !bytes.Equal(data, expected) {
// 		t.Errorf("Data = %q, want %q", data, expected)
// 	}

// 	if bytes.Contains(data, []byte("*")) {
// 		t.Errorf("Data contains corrupted bytes: %q", data)
// 	}
// }

// // Вспомогательная функция
// func getKeys(m map[string][]byte) []string {
// 	keys := make([]string, 0, len(m))
// 	for k := range m {
// 		keys = append(keys, k)
// 	}
// 	return keys
// }
