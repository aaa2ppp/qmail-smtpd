AUTO_QMAIL=$(PWD)/var/qmail
BIN=$(AUTO_QMAIL)/bin
EXT=`uname | grep -q NT && echo .exe`

.PHONY: run qmail-smtpd qmail-queue mktmpdir test1 addcr

run: qmail-queue mktmpdir
	AUTO_QMAIL=$(AUTO_QMAIL) go run .

mktmpdir:
	mkdir -p $(AUTO_QMAIL)/tmp

qmail-smtpd:
	go build -o $(BIN)/qmail-smtpd$(EXT) .

qmail-queue:
	go build -o $(BIN)/qmail-queue$(EXT) ./cmd/fake-qmail-queue

addcr:
	go build -o $(BIN)/addcr$(EXT) ./cmd/addcr

build: qmail-smtpd qmail-queue addcr mktmpdir

test1: build
	cat test1.txt | $(BIN)/addcr | AUTO_QMAIL=$(AUTO_QMAIL) QQ_OUT0=tmp/qq.out0 QQ_OUT1=tmp/qq.out1 $(BIN)/qmail-smtpd
