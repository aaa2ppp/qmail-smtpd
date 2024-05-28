AUTO_QMAIL=$(PWD)/var/qmail
BIN=$(AUTO_QMAIL)/bin
TMP=$(AUTO_QMAIL)/tmp
CONTROL=$(AUTO_QMAIL)/control
EXT=`uname | grep -q NT && echo .exe`

.PHONY: run qmail-smtpd qmail-queue mktmpdir test1 addcr

mkqmtree:
	mkdir -p $(AUTO_QMAIL) && cd $(AUTO_QMAIL) && mkdir -p control tmp bin

qmail-smtpd: mkqmtree
	go build -o $(BIN)/qmail-smtpd$(EXT) ./cmd/qmail-smtpd

qmail-queue: mkqmtree
	go build -o $(BIN)/qmail-queue$(EXT) ./cmd/fake-qmail-queue

addcr: mkqmtree
	go build -o $(BIN)/addcr$(EXT) ./cmd/addcr

build: qmail-smtpd qmail-queue addcr

test1: build
	cat test1.txt | $(BIN)/addcr | AUTO_QMAIL=$(AUTO_QMAIL) QQ_OUT0=tmp/qq.out0 QQ_OUT1=tmp/qq.out1 $(BIN)/qmail-smtpd

smtpd-cover:
	go test -coverprofile=$(TMP)/smtpd-cover.out ./internal/smtpd -run . && go tool cover -html $(TMP)/smtpd-cover.out
