package env

import (
	"testing"
)

func TestEnv_Set_Lookup(t *testing.T) {
	env := make(Env)
	env.Set("KEY1", "VALUE1")
	env.Set("KEY2", "VALUE2")

	if v, ok := env.Lookup("KEY1"); !ok || v != "VALUE1" {
		t.Errorf("Get(KEY1) = (%s, %t), want (VALUE1, true)", v, ok)
	}
	if v, ok := env.Lookup("KEY2"); !ok || v != "VALUE2" {
		t.Errorf("Get(KEY2) = (%s, %t), want (VALUE2, true)", v, ok)
	}
	if v, ok := env.Lookup("MISSING"); ok || v != "" {
		t.Errorf("Get(MISSING) = (%s, %t), want (\"\", false)", v, ok)
	}
}

func TestEnv_Append(t *testing.T) {
	env := make(Env)
	env.Append("KEY1=VALUE1", "KEY2=VALUE2", "INVALID", "KEY3=VALUE3")

	if v, ok := env.Lookup("KEY1"); !ok || v != "VALUE1" {
		t.Errorf("Get(KEY1) = (%s, %t), want (VALUE1, true)", v, ok)
	}
	if v, ok := env.Lookup("KEY2"); !ok || v != "VALUE2" {
		t.Errorf("Get(KEY2) = (%s, %t), want (VALUE2, true)", v, ok)
	}
	if v, ok := env.Lookup("KEY3"); !ok || v != "VALUE3" {
		t.Errorf("Get(KEY3) = (%s, %t), want (VALUE3, true)", v, ok)
	}
	if v, ok := env.Lookup("INVALID"); ok {
		t.Errorf("Get(INVALID) = (%s, %t), want (\"\", false)", v, ok)
	}
}

func TestEnv_Copy(t *testing.T) {
	src := make(Env)
	src.Set("A", "1")
	src.Set("B", "2")

	dst := make(Env)
	dst.Set("C", "3")
	dst.Copy(src)

	if v, ok := dst.Lookup("A"); !ok || v != "1" {
		t.Errorf("dst.Get(A) = (%s, %t), want (1, true)", v, ok)
	}
	if v, ok := dst.Lookup("B"); !ok || v != "2" {
		t.Errorf("dst.Get(B) = (%s, %t), want (2, true)", v, ok)
	}
	if v, ok := dst.Lookup("C"); !ok || v != "3" {
		t.Errorf("dst.Get(C) = (%s, %t), want (3, true)", v, ok)
	}
}

func TestEnv_Clone(t *testing.T) {
	env1 := make(Env)
	env1.Set("A", "1")
	env1.Set("B", "2")

	env2 := env1.Clone()

	env2.Set("C", "3")

	if v, ok := env1.Lookup("C"); ok {
		t.Errorf("env1.Get(C) = (%s, %t), want (\"\", false)", v, ok)
	}
	if v, ok := env2.Lookup("C"); !ok || v != "3" {
		t.Errorf("env2.Get(C) = (%s, %t), want (3, true)", v, ok)
	}
}

func TestEnv_Environ(t *testing.T) {
	env := make(Env)
	env.Set("A", "1")
	env.Set("B", "2")

	environ := env.Environ()

	expected := []string{"A=1", "B=2"}
	if len(environ) != len(expected) {
		t.Fatalf("Environ() length = %d, want %d", len(environ), len(expected))
	}

	for _, exp := range expected {
		found := false
		for _, v := range environ {
			if v == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Environ() missing expected value: %s", exp)
		}
	}
}

func TestEnv_GetMissing(t *testing.T) {
	env := make(Env)
	env.Set("A", "1")

	if v, ok := env.Lookup("MISSING"); ok || v != "" {
		t.Errorf("Get(MISSING) = (%s, %t), want (\"\", false)", v, ok)
	}
}
