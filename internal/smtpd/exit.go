package smtpd

type exitCode int

func _exit(code int) {
	panic(exitCode(code))
}
