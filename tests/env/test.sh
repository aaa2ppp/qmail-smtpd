#!/bin/sh

# set -x
set -e

usage() {
    echo "Usage: $(basename $0) [-v]" >&2
    exit 1
}

verbose=no
while getopts "v" optname; do
    case $optname in
    v) verbose=yes;;
    *) usage;;
    esac
done

script_dir="$(cd "$(dirname "$0")" && pwd)"

project_root="$script_dir/../.."
cmd="$project_root/cmd"

export AUTO_QMAIL="$script_dir/tmp/$$"

bin="$AUTO_QMAIL/bin"
tmp="$AUTO_QMAIL/tmp"
control="$AUTO_QMAIL/control"

test_expected="$tmp/test.expected"
test_actual="$tmp/test.actual"
server_log="$tmp/server.log"

echo "Make test bundle $AUTO_QMAIL">&2
mkdir -p $bin $tmp $control

mkdir -p "$tmp/qmail-queue"
cat > "$tmp/qmail-queue/main.go" << EOF
package main

import (
	"fmt"
	"log"
	"os"
)

var envs = []string{
	"EXTERNAL_VARIABLE", // внешня переменная
	"PROTO",             // устанавливает server -> 'TCP', smtpd -> 'ESMTP'
	"TCPLOCALIP",        // устанавливает server
	"TCPLOCALHOST",      //
	"TCPREMOTEIP",       //
	"TCPREMOTEHOST",     // устанавливает server, берет из DNS
	"TCPRULE1",          // устанавливает server из cdb
	"TCPRULE2",          //
	"TCPREMOTEINFO",     // установливает smtpd при успешной авторизации
}

func main() {
	log.SetPrefix("qmail-queue:")

	out, err := os.Create("tmp/test.actual")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	for _, name := range envs {
		if value, ok := os.LookupEnv(name); ok {
			if name == "TCPREMOTEHOST" {
				// replace real value to stub for test
				fmt.Fprintf(out, "%s=%q\n", name, "remote.host.stub")
				continue
			}
			fmt.Fprintf(out, "%s=%q\n", name, value)
		}
	}
}
EOF

mkdir -p "$tmp/vchkpw"
cat > "$tmp/vchkpw/main.go" << EOF
package main

import (
	"io"
	"os"
)

func main() {
	// Пробуем читать из fd3, если не получается - из stdin
	var r io.Reader = os.Stdin
	if f := os.NewFile(3, "fd3"); f != nil {
		r = f
	}
	io.Copy(io.Discard, r)
	os.Exit(0)
}
EOF

GOEXE=$(go env GOEXE)
go build -o "$bin/server$GOEXE"   "$cmd/server"
go build -o "$bin/tcprules$GOEXE" "$cmd/tcprules"
go build -o "$bin/netcat$GOEXE"   "$cmd/netcat"
go build -o "$bin/addcr$GOEXE"    "$cmd/addcr"

go build -o "$bin/qmail-queue$GOEXE" "$tmp/qmail-queue"
go build -o "$bin/vchkpw$GOEXE" "$tmp/vchkpw"

echo example.org > "$control/me"
echo 1           > "$control/smtplog"
echo 30          > "$control/timeoutsmtpd"
echo ":allow,TCPRULE1='rule1',TCPRULE2='rule2'" > "$control/tcp.2525"

"$bin/tcprules" "$control/tcp.2525.cdb" "$control/tcp.2525.$$" < "$control/tcp.2525"

cleanup() {
    set +e
    test -n "$SERVER_PID" && kill $SERVER_PID && wait $SERVER_PID >/dev/null 2>&1
    rm -fr "$AUTO_QMAIL"
}

trap cleanup EXIT

echo "Starting server..." >&2
export EXTERNAL_VARIABLE="this variable was defined before starting the server"
"$bin/server" -x "$control/tcp.2525.cdb" -h -addr 127.0.0.1:2525 "$bin/vchkpw" > "$server_log" 2>&1 &
SERVER_PID=$!

# wait server startup
sleep 1

echo "Running SMTP test session..." >&2
($bin/addcr << EOF
EHLO test.example.com
AUTH CRAM-MD5
dmFzeWFAcHVwa2luIEFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFB
MAIL FROM: <test@example.com>
RCPT TO: <recipient@example.org>
DATA
Subject: Test

Test message
.
QUIT
EOF
) | "$bin/netcat" --addr=127.0.0.1:2525 -t 1s > /dev/null

cat > "$test_expected" << EOF
EXTERNAL_VARIABLE="$EXTERNAL_VARIABLE"
PROTO="ESMTP"
TCPLOCALIP="127.0.0.1"
TCPLOCALHOST="example.org"
TCPREMOTEIP="127.0.0.1"
TCPREMOTEHOST="remote.host.stub"
TCPRULE1="rule1"
TCPRULE2="rule2"
TCPREMOTEINFO="vasya@pupkin"
EOF

if [ ! -f "$test_actual" ]; then
    echo "FAILED: No output file generated at $test_actual" >&2
    echo "Server logs:" >&2
    cat "$server_log" >&2
    return 1
fi

if [ "$verbose" = "yes" ]; then
    echo "=== Actual output ===" >&2
    cat "$test_actual" >&2
    echo "=== Expected output ===" >&2
    cat "$test_expected" >&2
fi
    
if diff -u "$test_expected" "$test_actual"; then
    echo "PASSED: Output matches expected result" >&2
    exit 0
else
    echo "FAILED: Output does not match expected result" >&2
    exit 1
fi
