package control

import (
	"testing"
)

// MockEngine для тестирования
type MockEngine struct {
	LineResponses map[string]struct {
		Value  string
		Exists bool
		Err    error
	}
	TextResponses map[string]struct {
		Lines  []string
		Exists bool
		Err    error
	}
}

func (m MockEngine) ReadLine(name string) (string, bool, error) {
	if resp, ok := m.LineResponses[name]; ok {
		return resp.Value, resp.Exists, resp.Err
	}
	return "", false, nil
}

func (m MockEngine) ReadText(name string) ([]string, bool, error) {
	if resp, ok := m.TextResponses[name]; ok {
		return resp.Lines, resp.Exists, resp.Err
	}
	return nil, false, nil
}

func TestControl_WithMock(t *testing.T) {
	mockEngine := MockEngine{
		LineResponses: map[string]struct {
			Value  string
			Exists bool
			Err    error
		}{
			"control/me":     {Value: "test.com", Exists: true},
			"control/exists": {Value: "file content", Exists: true},
		},
	}

	ctrl, err := New(mockEngine)
	if err != nil {
		t.Fatalf("NewWithEngine failed: %v", err)
	}

	// Тест ReadLineDef с моком
	result, err := ctrl.ReadLineDef("control/exists", false, "default")
	if err != nil {
		t.Fatalf("ReadLineDef failed: %v", err)
	}
	if result != "file content" {
		t.Errorf("got %q, want %q", result, "file content")
	}

	// Тест с несуществующим файлом
	result, err = ctrl.ReadLineDef("control/nonexistent", true, "default")
	if err != nil {
		t.Fatalf("ReadLineDef failed: %v", err)
	}
	if result != "test.com" { // Должен использовать me
		t.Errorf("got %q, want %q", result, "test.com")
	}
}

func TestControl_ReadText_WithMock(t *testing.T) {
	mockEngine := MockEngine{
		LineResponses: map[string]struct {
			Value  string
			Exists bool
			Err    error
		}{
			"control/me": {Value: "fallback.com", Exists: true},
		},
		TextResponses: map[string]struct {
			Lines  []string
			Exists bool
			Err    error
		}{
			"control/multiline": {
				Lines:  []string{"line1", "line2"},
				Exists: true,
			},
		},
	}

	ctrl, err := New(mockEngine)
	if err != nil {
		t.Fatalf("NewWithEngine failed: %v", err)
	}

	// Тест чтения многострочного файла
	lines, err := ctrl.ReadText("control/multiline", false)
	if err != nil {
		t.Fatalf("ReadText failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2", len(lines))
	}

	// Тест fallback на me
	lines, err = ctrl.ReadText("control/nonexistent", true)
	if err != nil {
		t.Fatalf("ReadText failed: %v", err)
	}
	if len(lines) != 1 || lines[0] != "fallback.com" {
		t.Errorf("got %v, want [fallback.com]", lines)
	}
}
