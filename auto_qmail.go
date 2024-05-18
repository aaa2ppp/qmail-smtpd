package main

import "os"

var auto_qmail = "/var/qmail"

func init() {
	v := os.Getenv("AUTO_QMAIL")
	if v != "" {
		auto_qmail = v
	}
}
