#!/bin/sh

: ${AUTO_QMAIL?}

bin=$AUTO_QMAIL/bin
tmp="$(cd "$(dirname $0)"; pwd)/tmp"

mkdir -p $tmp

client_say() {
    $bin/addcr << EOF 
helo
ehlo localhost
rcpt <pasha>
rset
mail <masha>
data
rset
mail <pasha>
rcpt <yasha>
rset
mail <masha>
rcpt <yasha>
data
From: "Masha" <masha>
To: "Yasha" <yasha>
Subject: Hello

Hello, Yasha!
.
rset
quit
EOF
}

client_say | \
QMAILQUEUE="bin/fake-qmail-queue" \
TCPLOCALIP="192.168.1.25" \
TCPREMOTEIP="192.168.2.101" \
QQ_OUT0="$tmp/qq.out0" \
QQ_OUT1="$tmp/qq.out1" \
$bin/qmail-smtpd > $tmp/responses.txt 2> $tmp/session.log
