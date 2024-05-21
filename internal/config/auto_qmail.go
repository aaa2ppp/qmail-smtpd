package config

import "os"

var AutoQmail = "/var/qmail"

func init() {
	v := os.Getenv("AUTO_QMAIL")
	if v != "" {
		AutoQmail = v
	}
}
