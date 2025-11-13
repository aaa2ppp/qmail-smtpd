#!/bin/sh

go build -o ./bin/tcprules ../../cmd/tcprules  || exit 1
mkdir -p ./tmp


# 1. Создаем CDB
./bin/tcprules -size=65536 ./tmp/test.cdb ./tmp/test.tmp < rules.txt || exit 1

# 2. Дампим обратно для проверки
./bin/tcprules -dump ./tmp/test.cdb > ./tmp/test.txt

# 3. Проверяем конкретные IP
./bin/tcprules -dump ./tmp/test.cdb | grep "127.0.0.1"
./bin/tcprules -dump ./tmp/test.cdb | grep "192.168.1.1"
./bin/tcprules -dump ./tmp/test.cdb | grep "1.2.3.4"
