package safeio

import (
	"bytes"
	"testing"
)

func TestFlushEmptyBuffer(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newLogWriter(buf, "")

	err := writer.Flush()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("Expected no output for empty buffer, got: %s", buf.String())
	}
}

func TestFlushSingleLine(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newLogWriter(buf, "SMTP: ")

	writer.WriteString("EHLO example.com")
	err := writer.Flush()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "SMTP: EHLO example.com"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

func TestFlushMultipleLines(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newLogWriter(buf, "> ")

	writer.WriteString("line 1\nline 2\nline 3")
	err := writer.Flush()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "> line 1\n> line 2\n> line 3"

	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

func TestFlushEscapeSpecialCharacters(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := newLogWriter(buf, "LOG: ")

	writer.WriteString("line with \"quotes\" and\ttab and\r\nnewline")
	err := writer.Flush()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "LOG: line with \"quotes\" and\ttab and\r\nLOG: newline"

	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}
