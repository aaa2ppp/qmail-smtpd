package server

import (
	"testing"
)

func TestSemaphorePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on double release, but got none")
		} else {
			t.Logf("Got expected panic: %v", r)
		}
	}()

	sem := makeSemaphore(1)

	// Занимаем слот
	if !sem.acquire() {
		t.Fatal("Failed to acquire")
	}

	// Освобождаем - ок
	sem.release()

	// ЕЩЕ РАЗ освобождаем - ДОЛЖНА БЫТЬ ПАНИКА!
	sem.release()
}

func TestSemaphoreEdgeCases(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for n <= 0")
		}
	}()

	// Нельзя создать семафор с n <= 0
	_ = makeSemaphore(0)
}

func TestSemaphoreNilSafety(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unexpected panic: %v", r)
		}
	}()

	var sem semaphore

	// Не должно паниковать на nil семафоре
	sem.acquire() // true
	sem.release() // no panic
}
