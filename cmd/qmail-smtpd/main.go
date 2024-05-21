package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/smtpd"
)

func main() {
	// void sig_pipeignore() { sig_catch(SIGPIPE,SIG_IGN); }
	signal.Ignore(syscall.SIGPIPE)
	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}
	smtpd.Run()
}
