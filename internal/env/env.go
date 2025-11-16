package env

import (
	"maps"
	"strings"
)

// Env представляет переменные окружения в формате "KEY=VALUE".
//
// Хранит полные строки "KEY=VALUE" для оптимизации:
//   - Append проверяет наличие '=' и сохраняет строку без дополнительного парсинга
//   - Environ возвращает строки без сборки
//   - Set собирает строку один раз при установке
//
// Это минимизирует аллокации при частых операциях с окружением.
type Env map[string]string // key -> "key=value"

func New(envs []string) Env {
	e := make(Env, len(envs))
	e.Append(envs...)
	return e
}

// Append добавляет переменные окружения из слайса []string вида "name=value".
func (e Env) Append(envs ...string) {
	for _, env := range envs {
		if p := strings.IndexByte(env, '='); p != -1 {
			e[env[:p]] = env
		}
	}
}

// Copy копирует все переменные окружения из src в ресивер.
// Переменные с одинаковыми именами перезаписываются.
func (e Env) Copy(src Env) {
	maps.Copy(e, src)
}

func (e Env) Clone() Env {
	return maps.Clone(e)
}

func (e Env) Contains(name string) bool {
	_, ok := e[name]
	return ok
}

func (e Env) Set(name, value string) {
	var buf strings.Builder
	n := len(name) + len(value) + 1
	buf.Grow(n)
	buf.WriteString(name)
	buf.WriteByte('=')
	buf.WriteString(value)
	env := buf.String()
	e[env[:len(name)]] = env
}

func (e Env) Unset(name string) {
	delete(e, name)
}

func (e Env) Get(name string) string {
	val, _ := e.Lookup(name)
	return val
}

func (e Env) Lookup(name string) (string, bool) {
	if env, ok := e[name]; ok {
		return env[len(name)+1:], true
	}
	return "", false
}

func (e Env) Environ() []string {
	envs := make([]string, 0, len(e))
	for _, env := range e {
		envs = append(envs, env)
	}
	return envs
}
