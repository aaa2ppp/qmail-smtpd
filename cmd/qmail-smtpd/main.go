package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"qmail-smtpd/internal/config"
	"qmail-smtpd/internal/qmail"
	"qmail-smtpd/internal/smtpd"
)

type qmailAdaptor struct{}

func (qa qmailAdaptor) Open() (smtpd.QmailQueue, error) {
	return qmail.Open()
}

func main() {
	// void sig_pipeignore() { sig_catch(SIGPIPE,SIG_IGN); }
	signal.Ignore(syscall.SIGPIPE)
	if err := os.Chdir(config.AutoQmail); err != nil {
		log.Fatal(err)
	}
	var sd smtpd.Smtpd
	if err := sd.Run(os.Stdin, os.Stdout, qmailAdaptor{}); err != nil {
		log.Fatal(err)
	}
}
