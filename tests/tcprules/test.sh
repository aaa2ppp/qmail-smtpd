#!/bin/sh

: ${AUTO_QMAIL?}

bin=$AUTO_QMAIL/bin
tmp=$(dirname $0)/tmp
rules=$(dirname $0)/rules.txt

# 1. Создаем CDB
$bin/tcprules -size=65536 $tmp/test.cdb $tmp/test.tmp < $rules || exit 1

# 2. Дампим обратно для проверки
$bin/tcprules -dump $tmp/test.cdb > $tmp/test.txt || exit 1

# 3. Проверяем конкретные IP
$bin/tcprules -dump $tmp/test.cdb | grep "127.0.0.1"
$bin/tcprules -dump $tmp/test.cdb | grep "192.168.1.1"
$bin/tcprules -dump $tmp/test.cdb | grep "1.2.3.4"
