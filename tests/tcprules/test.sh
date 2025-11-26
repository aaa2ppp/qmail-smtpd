#!/bin/sh

: ${AUTO_QMAIL?}

bin=$AUTO_QMAIL/bin
tmp=$(dirname $0)/tmp
rules=$(dirname $0)/rules.txt

mkdir -p $tmp

set -e

# 1. Создаем CDB
$bin/tcprules -size=65536 $tmp/test.cdb $tmp/test.tmp << EOF
# Базовые правила доступа
127.0.0.1:allow
192.168.1.:allow
10.:deny

# Правила с переменными окружения
1.2.3.4:allow,RELAYCLIENT="1"
5.6.7.8:allow,RELAYCLIENT="1",TCPLOCALIP="1.2.3.4"

# Правила с разными кавычками
8.8.8.8:allow,NAME='google',DNS='8.8.8.8'
9.9.9.9:allow,NAME=!quad9!,DNS=!9.9.9.9!
1.1.1.1:allow,NAME=|cloudflare|,DNS=|1.1.1.1|

# Подсети
172.16-31.:deny,REASON="private"
169.254.0.:deny,REASON="link-local"

# Хосты
=example.org:allow,RELAYCLIENT=""
=.org:deny

# Правило по умолчанию (должно быть последним)
:deny,REASON="default_deny"
EOF

# 2. Дампим обратно для проверки
$bin/tcprules -dump $tmp/test.cdb > $tmp/test.txt

# 3. Проверяем конкретные IP
$bin/tcprules -dump $tmp/test.cdb | grep "127.0.0.1"
$bin/tcprules -dump $tmp/test.cdb | grep "192.168.1."
$bin/tcprules -dump $tmp/test.cdb | grep "10."
$bin/tcprules -dump $tmp/test.cdb | grep "1.2.3.4"
$bin/tcprules -dump $tmp/test.cdb | grep "172.17."
$bin/tcprules -dump $tmp/test.cdb | grep "169.254.0."
$bin/tcprules -dump $tmp/test.cdb | grep "=example.org"
$bin/tcprules -dump $tmp/test.cdb | grep "=.org"
$bin/tcprules -dump $tmp/test.cdb | egrep "^:deny"
