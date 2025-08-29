package qmail

import (
	"os"
	"strings"
)

// TODO: qmail-smtpd does not filter environment.
// It sets TCPLOCALIP, TCPREMOTEIP, RELAYCLIENT, etc.
// and then execs qmail-queue with the full current environment.
// We must preserve all existing env vars, only override qmail-specific ones.

type QmailEnv struct {
	LocalIP       string
	LocalHost     string
	RemoteIP      string
	RemoteHost    string
	RemoteInfo    string
	Proto         string
	RelayClient   string
	RelayClientOk bool
	Authorized    bool
}

var envSet = map[string]bool{
	"TCPLOCALIP":    true,
	"TCPLOCALHOST":  true,
	"TCPREMOTEIP":   true,
	"TCPREMOTEHOST": true,
	"TCPREMOTEINFO": true,
	"PROTO":         true,
	"RELAYCLIENT":   true,
	"QMAILQUEUE":    true,
}

func prepareEnv(opts QmailEnv) []string {
	// Начинаем с текущего окружения
	env := os.Environ()

	// Удаляем переменные, которые мы хотим оверрайдить
	env = filterEnv(env, envSet)

	// Добавляем свои
	env = append(env,
		"TCPLOCALIP="+opts.LocalIP,
		"TCPLOCALHOST="+opts.LocalHost,
		"TCPREMOTEIP="+opts.RemoteIP,
		"TCPREMOTEHOST="+opts.RemoteHost,
		"PROTO="+opts.Proto,
	)

	if opts.Authorized {
		env = append(env, "TCPREMOTEINFO="+opts.RemoteInfo)
	}

	if opts.RelayClientOk {
		env = append(env, "RELAYCLIENT="+opts.RelayClient)
	}

	// QMAILQUEUE — наследуется извне, если есть.
	// Imho: это не обязательно (можно не отфильтровывать), зато явно
	if v, ok := os.LookupEnv("QMAILQUEUE"); ok {
		env = append(env, "QMAILQUEUE="+v)
	}

	return env
}

// filterEnv удаляет переменные окужения из env заданные keys.
// Внимание: функция изменяет исходный слайс - удаляет на месте.
// Возвращает слайс указывающий на туже самую память. Исходный порядок не гарантирован.
func filterEnv(env []string, keys map[string]bool) []string {
	for i := len(env) - 1; i >= 0; i-- {
		if p := strings.IndexByte(env[i], '='); p != -1 && keys[env[i][:p]] {
			n := len(env)
			env[i] = env[n-1]
			env[n-1] = ""
			env = env[:n-1]
		}
	}
	return env
}
