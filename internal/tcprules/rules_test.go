package tcprules

import (
	"errors"
	"reflect"
	"testing"
)

// MockDB реализует интерфейс DB для тестирования
type MockDB struct {
	data map[string]string
}

func NewMockDB(data map[string]string) *MockDB {
	return &MockDB{data: data}
}

func (m *MockDB) Do(fn func(Getter) error) error {
	return fn(m)
}

func (m *MockDB) Get(key string) (string, error) {
	val, ok := m.data[key]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

func TestTCPRules_getByIPPrefixes(t *testing.T) {
	type args struct {
		ip string
	}
	tests := []struct {
		name       string
		dbData     map[string]string
		args       args
		wantRule   string
		wantErr    error
	}{
		{
			name: "find longest prefix",
			dbData: map[string]string{
				"192.168.1.": "D\x00",
				"192.168.":   "+NET=internal\x00",
			},
			args:     args{ip: "192.168.1.100"},
			wantRule: "D\x00",
			wantErr:  nil,
		},
		{
			name: "find shorter prefix",
			dbData: map[string]string{
				"192.168.": "+NET=internal\x00",
			},
			args:     args{ip: "192.168.1.100"},
			wantRule: "+NET=internal\x00",
			wantErr:  nil,
		},
		{
			name: "find no prefix match",
			dbData: map[string]string{
				"10.0.0.": "+NET=other\x00",
			},
			args:     args{ip: "192.168.1.100"},
			wantRule: "",
			wantErr:  ErrNotFound,
		},
		{
			name: "no dots in IP",
			dbData: map[string]string{
				"192": "+NET=none\x00",
			},
			args:     args{ip: "192"},
			wantRule: "",
			wantErr:  ErrNotFound, // 192 has no dots, so no prefixes like 192. will be searched
		},
		{
			name: "IP ends with dot",
			dbData: map[string]string{
				"192.168.1.": "+NET=ends_with_dot\x00",
			},
			args:     args{ip: "192.168.1."},
			wantRule: "+NET=ends_with_dot\x00",
			wantErr:  nil,
		},
		{
			name: "empty IP",
			dbData: map[string]string{
				"": "+NET=default\x00",
			},
			args:     args{ip: ""},
			wantRule: "",
			wantErr:  ErrNotFound, // "" is not a prefix, it's the default rule handled elsewhere
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewMockDB(tt.dbData)
			rules := New(db)
			var gotRule string
			err := rules.db.Do(func(q Getter) error {
				var internalErr error
				gotRule, internalErr = rules.getByIPPrefixes(q, tt.args.ip)
				return internalErr
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("getByIPPrefixes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && gotRule != tt.wantRule {
				t.Errorf("getByIPPrefixes() rule = %v, want %v", gotRule, tt.wantRule)
			}
		})
	}
}

func TestTCPRules_getByHostSuffixes(t *testing.T) {
	type args struct {
		fqdn string
	}
	tests := []struct {
		name       string
		dbData     map[string]string
		args       args
		wantRule   string
		wantErr    error
	}{
		{
			name: "find longest suffix",
			dbData: map[string]string{
				"=.example.com": "D\x00",
				"=.com":         "+TLD=allowed\x00",
			},
			args:     args{fqdn: "sub.example.com"},
			wantRule: "D\x00",
			wantErr:  nil,
		},
		{
			name: "find shorter suffix",
			dbData: map[string]string{
				"=.com": "+TLD=allowed\x00",
			},
			args:     args{fqdn: "sub.example.com"},
			wantRule: "+TLD=allowed\x00",
			wantErr:  nil,
		},
		{
			name: "find no suffix match",
			dbData: map[string]string{
				"=.org": "+TLD=other\x00",
			},
			args:     args{fqdn: "sub.example.com"},
			wantRule: "",
			wantErr:  ErrNotFound,
		},
		{
			name: "no dots in FQDN",
			dbData: map[string]string{
				"=host": "+HOST=single\x00",
			},
			args:     args{fqdn: "host"},
			wantRule: "",
			wantErr:  ErrNotFound, // =host is not a suffix starting with a dot
		},
		{
			name: "FQDN starts with dot",
			dbData: map[string]string{
				"=.host": "+HOST=leading_dot\x00",
			},
			args:     args{fqdn: ".host"}, // Technically valid, though unusual input
			wantRule: "+HOST=leading_dot\x00",
			wantErr:  nil,
		},
		{
			name: "single label FQDN",
			dbData: map[string]string{
				"=.com": "+TLD=allowed\x00",
			},
			args:     args{fqdn: "com"}, // No dots, no suffixes starting with '.'
			wantRule: "",
			wantErr:  ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewMockDB(tt.dbData)
			rules := New(db)
			var gotRule string
			err := rules.db.Do(func(q Getter) error {
				var internalErr error
				gotRule, internalErr = rules.getByHostSuffixes(q, tt.args.fqdn)
				return internalErr
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("getByHostSuffixes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && gotRule != tt.wantRule {
				t.Errorf("getByHostSuffixes() rule = %v, want %v", gotRule, tt.wantRule)
			}
		})
	}
}

func TestTCPRules_GetByIPHost(t *testing.T) {
	type args struct {
		ip   string
		host string
	}
	tests := []struct {
		name       string
		dbData     map[string]string
		args       args
		wantResult Result
		wantErr    error
	}{
		{
			// 1. IP exact match
			name: "IP exact match wins over host",
			dbData: map[string]string{
				"192.168.1.100": "D\x00",
				"=example.com":  "+HOST=allowed\x00",
			},
			args: args{ip: "192.168.1.100", host: "example.com"},
			wantResult: Result{
				Allow: false,
				Env:   nil,
			},
			wantErr: nil,
		},
		{
			// 2. Host exact match (if IP not found)
			name: "Host exact match",
			dbData: map[string]string{
				"=example.com": "+HOST=allowed\x00",
			},
			args: args{ip: "192.168.1.100", host: "example.com"},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"HOST": "allowed"},
			},
			wantErr: nil,
		},
		{
			// 3. IP prefix match
			name: "IP prefix match",
			dbData: map[string]string{
				"192.168.1.": "+NET=internal\x00",
			},
			args: args{ip: "192.168.1.100", host: "example.com"},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"NET": "internal"},
			},
			wantErr: nil,
		},
		{
			// 4. Host suffix match (if IP and host exact not found)
			name: "Host suffix match",
			dbData: map[string]string{
				"=.com": "+TLD=allowed\x00",
			},
			args: args{ip: "192.168.1.100", host: "sub.example.com"},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"TLD": "allowed"},
			},
			wantErr: nil,
		},
		{
			// 5. Host default match (if host suffixes not found, but host was provided)
			name: "Host default match",
			dbData: map[string]string{
				"=": "+DEFAULT_HOST=1\x00",
			},
			args: args{ip: "192.168.1.100", host: "sub.example.com"},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"DEFAULT_HOST": "1"},
			},
			wantErr: nil,
		},
		{
			// 6. Default match (if all else fails)
			name: "Default match",
			dbData: map[string]string{
				"": "+DEFAULT=1\x00",
			},
			args: args{ip: "192.168.1.100", host: "sub.example.com"},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"DEFAULT": "1"},
			},
			wantErr: nil,
		},
		{
			// 7. No host provided, only IP logic applies
			name: "No host provided",
			dbData: map[string]string{
				"192.168.1.100": "+USER=local\x00",
				"192.168.1.":   "+NET=internal\x00",
				"=example.com":  "+HOST=allowed\x00", // Should not be found
			},
			args: args{ip: "192.168.1.100", host: ""},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"USER": "local"},
			},
			wantErr: nil,
		},
		{
			// 8. No rules found
			name: "No rules found",
			dbData: map[string]string{
				"10.0.0.1": "+NET=other\x00",
				"=other.com": "+HOST=other\x00",
			},
			args:       args{ip: "192.168.1.100", host: "example.com"},
			wantResult: Result{},
			wantErr:    ErrNotFound,
		},
		{
			// 9. Host provided but empty IP
			name: "Empty IP, Host exact match",
			dbData: map[string]string{
				"=example.com": "+HOST=allowed\x00",
			},
			args: args{ip: "", host: "example.com"},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"HOST": "allowed"},
			},
			wantErr: nil,
		},
		{
			// 10. Host provided, IP prefix match
			name: "Host provided, IP prefix match",
			dbData: map[string]string{
				"192.168.": "+NET=internal\x00",
			},
			args: args{ip: "192.168.1.100", host: "example.com"},
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{"NET": "internal"},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewMockDB(tt.dbData)
			rules := New(db)

			result, err := rules.GetByIPHost(tt.args.ip, tt.args.host)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetByIPHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if result.Allow != tt.wantResult.Allow {
					t.Errorf("GetByIPHost() Allow = %v, want %v", result.Allow, tt.wantResult.Allow)
				}
				if len(result.Env) != len(tt.wantResult.Env) {
					t.Errorf("GetByIPHost() Env length = %d, want %d", len(result.Env), len(tt.wantResult.Env))
				}
				for key, wantValue := range tt.wantResult.Env {
					gotValue, exists := result.Env[key]
					if !exists || gotValue != wantValue {
						t.Errorf("GetByIPHost() Env[%s] = %v, want %v", key, gotValue, wantValue)
					}
				}
			}
		})
	}
}

func TestTCPRules_Get(t *testing.T) {
	tests := []struct {
		name       string
		dbData     map[string]string
		ip         string
		wantResult Result
		wantErr    error
	}{
		{
			name: "exact IP match - deny",
			dbData: map[string]string{
				"192.168.1.100": "D\x00",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: false,
				Env:   nil,
			},
			wantErr: nil,
		},
		{
			name: "exact IP match - allow with env",
			dbData: map[string]string{
				"192.168.1.100": "+USER=test\x00+ROLE=admin\x00",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: true,
				Env: map[string]string{
					"USER": "test",
					"ROLE": "admin",
				},
			},
			wantErr: nil,
		},
		{
			name: "exact IP match - allow without env",
			dbData: map[string]string{
				"192.168.1.100": "",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{},
			},
			wantErr: nil,
		},
		{
			name: "exact IP match - allow without env \x00",
			dbData: map[string]string{
				"192.168.1.100": "\x00",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{},
			},
			wantErr: nil,
		},
		{
			name: "exact IP match - allow without env \x00\x00",
			dbData: map[string]string{
				"192.168.1.100": "\x00\x00",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{},
			},
			wantErr: nil,
		},
		{
			name: "subnet match - longest prefix",
			dbData: map[string]string{
				"192.168.1.": "D\x00",
				"192.168.":   "+NETWORK=internal\x00",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: false,
				Env:   nil,
			},
			wantErr: nil,
		},
		{
			name: "subnet match - shorter prefix",
			dbData: map[string]string{
				"192.168.": "+NETWORK=internal\x00",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: true,
				Env: map[string]string{
					"NETWORK": "internal",
				},
			},
			wantErr: nil,
		},
		{
			name: "default rule (empty string)",
			dbData: map[string]string{
				"": "D\x00",
			},
			ip: "192.168.1.100",
			wantResult: Result{
				Allow: false,
				Env:   nil,
			},
			wantErr: nil,
		},
		{
			name:    "no rules found",
			dbData:  map[string]string{},
			ip:      "192.168.1.100",
			wantErr: ErrNotFound,
		},
		{
			name: "empty IP",
			dbData: map[string]string{
				"": "D\x00",
			},
			ip: "",
			wantResult: Result{
				Allow: false,
				Env:   nil,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewMockDB(tt.dbData)
			rules := New(db)

			result, err := rules.GetByIP(tt.ip)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if result.Allow != tt.wantResult.Allow {
					t.Errorf("Get() Allow = %v, want %v", result.Allow, tt.wantResult.Allow)
				}

				if len(result.Env) != len(tt.wantResult.Env) {
					t.Errorf("Get() Env length = %d, want %d", len(result.Env), len(tt.wantResult.Env))
				}

				for key, wantValue := range tt.wantResult.Env {
					gotValue, exists := result.Env[key]
					if !exists || gotValue != wantValue {
						t.Errorf("Get() Env[%s] = %v, want %v", key, gotValue, wantValue)
					}
				}
			}
		})
	}
}

func TestParseRule(t *testing.T) {
	tests := []struct {
		name       string
		rule       string
		wantResult Result
		wantErr    error
	}{
		{
			name: "deny rule",
			rule: "D\x00",
			wantResult: Result{
				Allow: false,
				Env:   nil,
			},
			wantErr: nil,
		},
		{
			name: "allow rule with env variables",
			rule: "+USER=test\x00+ROLE=admin\x00",
			wantResult: Result{
				Allow: true,
				Env: map[string]string{
					"USER": "test",
					"ROLE": "admin",
				},
			},
			wantErr: nil,
		},
		{
			name: "empty rule",
			rule: "",
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{},
			},
			wantErr: nil,
		},
		{
			name: "empty rule ends with \x00",
			rule: "\x00",
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{},
			},
			wantErr: nil,
		},
		{
			name: "empty rule ends with \x00\x00",
			rule: "\x00\x00",
			wantResult: Result{
				Allow: true,
				Env:   map[string]string{},
			},
			wantErr: nil,
		},
		{
			name:       "invalid format - missing null terminator",
			rule:       "D",
			wantResult: Result{},
			wantErr:    ErrInvalidFormat,
		},
		{
			name:       "invalid format - env without equals",
			rule:       "+USER\x00",
			wantResult: Result{},
			wantErr:    ErrInvalidFormat,
		},
		{
			name: "mixed rules - deny takes precedence",
			rule: "D\x00+USER=test\x00",
			wantResult: Result{
				Allow: false,
				Env:   nil,
			},
			wantErr: nil,
		},
		{
			name: "env variable with empty value",
			rule: "+USER=\x00",
			wantResult: Result{
				Allow: true,
				Env: map[string]string{
					"USER": "",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseBinRule(tt.rule)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("parseRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if result.Allow != tt.wantResult.Allow {
					t.Errorf("parseRule() Allow = %v, want %v", result.Allow, tt.wantResult.Allow)
				}

				if len(result.Env) != len(tt.wantResult.Env) {
					t.Errorf("parseRule() Env length = %d, want %d", len(result.Env), len(tt.wantResult.Env))
				}

				for key, wantValue := range tt.wantResult.Env {
					gotValue, exists := result.Env[key]
					if !exists || gotValue != wantValue {
						t.Errorf("parseRule() Env[%s] = %v, want %v", key, gotValue, wantValue)
					}
				}
			}
		})
	}
}

func TestTCPRules_Get_Priority(t *testing.T) {
	// Тест приоритета: более специфичные правила должны иметь приоритет
	db := NewMockDB(map[string]string{
		"192.168.1.100": "+HOST=specific\x00", // самый специфичный
		"192.168.1.":    "+HOST=subnet1\x00",  // менее специфичный
		"192.168.":      "+HOST=subnet2\x00",  // еще менее специфичный
		"192.":          "+HOST=subnet3\x00",  // еще менее специфичный
		"":              "+HOST=default\x00",  // наименее специфичный
	})

	tests := []struct {
		name string
		ip   string
		want Result
	}{
		{
			"specific",
			"192.168.1.100",
			Result{
				Allow: true,
				Env:   map[string]string{"HOST": "specific"},
			},
		},
		{
			"subnet1",
			"192.168.1.101",
			Result{
				Allow: true,
				Env:   map[string]string{"HOST": "subnet1"},
			},
		},
		{
			"subnet2",
			"192.168.2.100",
			Result{
				Allow: true,
				Env:   map[string]string{"HOST": "subnet2"},
			},
		},
		{
			"subnet3",
			"192.169.1.100",
			Result{
				Allow: true,
				Env:   map[string]string{"HOST": "subnet3"},
			},
		},
		{
			"default",
			"193.168.1.100",
			Result{
				Allow: true,
				Env:   map[string]string{"HOST": "default"},
			},
		},
	}

	rules := New(db)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rules.GetByIP(tt.ip)
			if err != nil {
				t.Fatalf("Get() unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// Перехватываем вызовы Get чтобы отследить порядок
type trackingDB struct {
	order *[]string
}

func (t *trackingDB) Get(key string) (string, error) {
	*t.order = append(*t.order, key)
	return "", ErrNotFound
}

func (t *trackingDB) Do(fn func(Getter) error) error {
	return fn(t)
}

func TestTCPRules_Get_IPSearchOrder(t *testing.T) {
	// Тест порядка поиска: от полного IP до пустой строки
	searchOrder := []string{}

	trackingDB := trackingDB{&searchOrder}

	rules := New(&trackingDB)
	rules.GetByIP("192.168.1.100")

	expectedOrder := []string{
		"192.168.1.100",
		"192.168.1.",
		"192.168.",
		"192.",
		"",
	}

	if len(searchOrder) != len(expectedOrder) {
		t.Errorf("Search order length mismatch: got %d, want %d", len(searchOrder), len(expectedOrder))
	}

	for i, expected := range expectedOrder {
		if i >= len(searchOrder) {
			break
		}
		if searchOrder[i] != expected {
			t.Errorf("Search order[%d] = %s, want %s", i, searchOrder[i], expected)
		}
	}
}
