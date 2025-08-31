package qmail

import (
	"os"
	"strconv"
	"strings"
)

// TODO: qmail-smtpd does not filter environment.
// It sets TCPLOCALIP, TCPREMOTEIP, RELAYCLIENT, etc.
// and then execs qmail-queue with the full current environment.
// We must preserve all existing env vars, only override qmail-specific ones.

type Env struct {
	LocalIP    string
	LocalHost  string
	RemoteIP   string
	RemoteHost string
	Proto      string

	// RemoteInfo contains the authenticated username in the same way as in qmail-smtpd with auth patch.
	// It set only after successful AUTH and is never set externally. Before calling `qmail-queue`,
	// the TCPREMOTEINFO environment variable will be set from it.
	RemoteInfo string

	RelayClient   string
	RelayClientOk bool
	Databytes     int
	QmailQueue    string
}

var envSet = map[string]bool{
	"TCPLOCALIP":    true,
	"TCPLOCALHOST":  true,
	"TCPREMOTEIP":   true,
	"TCPREMOTEHOST": true,
	"TCPREMOTEINFO": true,
	"PROTO":         true,
	"RELAYCLIENT":   true,
	"DATABYTES":     true,
	"QMAILQUEUE":    true,
}

func (env *Env) Prepare() []string {
	// Начинаем с текущего окружения
	osEnv := os.Environ()

	// Удаляем переменные, которые мы хотим оверрайдить
	osEnv = filterEnv(osEnv, envSet)

	// Добавляем свои
	osEnv = append(osEnv,
		"TCPLOCALIP="+env.LocalIP,
		"TCPLOCALHOST="+env.LocalHost,
		"TCPREMOTEIP="+env.RemoteIP,
		"TCPREMOTEHOST="+env.RemoteHost,
		"PROTO="+env.Proto,
	)

	if env.RemoteInfo != "" {
		osEnv = append(osEnv, "TCPREMOTEINFO="+env.RemoteInfo)
	}

	if env.Databytes > 0 {
		osEnv = append(osEnv, "DATABYTES="+strconv.Itoa(env.Databytes))
	}

	if env.QmailQueue != "" {
		osEnv = append(osEnv, "QMAILQUEUE="+env.QmailQueue)
	}

	return osEnv
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
