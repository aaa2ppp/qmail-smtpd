#!/bin/sh

set -e

: ${LOCAL:?must be defined}

bin="/usr/local/bin"
qmail="/var/qmail"
vpopmail=${VPOPMAIL:-"/home/vpopmail"}

test -e "$bin/my-qmail-smtpd"
test -d "$qmail/bin"
test -d "$vpopmail/bin"

qmaild_uid=$(id -u qmaild)
qmaild_gid=$(id -g qmaild)

[ -d "$qmail/control"               ] || mkdir "$qmail/control"
[ -f "$qmail/control/me"            ] || echo  "$LOCAL" > "$qmail/control/me"
[ -f "$qmail/control/defaultdomain" ] || echo  "$LOCAL" > "$qmail/control/defaultdomain"
[ -f "$qmail/control/plusdomain"    ] || echo  "$LOCAL" > "$qmail/control/plusdomain"
# пустой locals, иначе будет использоваться me (vadddomain $LOCAL сам все настроит)
[ -f "$qmail/control/locals"        ] || touch "$qmail/control/locals"
# пустрой rcpthosts, иначе будет открытый релей
[ -f "$qmail/control/rcpthosts"     ] || touch "$qmail/control/rcpthosts"

# заворачиваем доставку на $LOCAL в vpopmail
if ! "$vpopmail/bin/vdominfo" "$LOCAL" >/dev/null 2>&1; then
    if [ -n "$PASSWORD" ]; then
        "$vpopmail/bin/vadddomain" "$LOCAL" "$PASSWORD"
    else
        "$vpopmail/bin/vadddomain" -r "$LOCAL"
    fi
fi

graceful_shutdown() {
    echo "Received shutdown signal, stopping processes..."
    if [ -n "$smtpd_pid" ]; then
        echo "Terminate smtpd... "
        kill -TERM "$smtpd_pid" && wait "$smtpd_pid" && echo ok || true
    fi
    if [ -n "$qmail_pid" ]; then
        echo "Terminate qmail... "
        kill -TERM "$qmail_pid" && wait "$qmail_pid" && echo ok || true
    fi
}

trap graceful_shutdown TERM INT

export PATH="/var/qmail/bin:$PATH"

case ${RUN_MODE:-native} in
    original)
        echo "Running tcpserver + original /var/qmail/bin/qmail-smtpd"
        "$qmail/bin/qmail-start" ./Maildir/ &
        qmail_pid=$!
        /usr/bin/tcpserver -v -R -H -l "$LOCAL" -u "$qmaild_uid" -g "$qmaild_gid" 0 25 \
            "$qmail/bin/qmail-smtpd" \
            "$LOCAL" "$vpopmail/bin/vchkpw" true &
        smtpd_pid=$!
        wait
        ;;
    compat)
        echo "Running in qmail-compatible mode (with tcpserver)"
        "$qmail/bin/qmail-start" ./Maildir/ &
        qmail_pid=$!
        /usr/bin/tcpserver -v -R -H -l "$LOCAL" -u "$qmaild_uid" -g "$qmaild_gid" 0 25 \
            "$bin/my-qmail-smtpd" \
            "$LOCAL" "$vpopmail/bin/vchkpw" true &
        smtpd_pid=$!
        wait
        ;;
    native)
        echo "Running in native server mode"
        "$qmail/bin/qmail-start" ./Maildir/ &
        qmail_pid=$!
        "$bin/my-qmail-smtpd" --addr=:25 -l "$LOCAL" \
            "$vpopmail/bin/vchkpw" true &
        smtpd_pid=$!
        wait
        ;;
    maintenance)
        echo "Running in maintenance mode"
        while true; do sleep 3600; done
        ;;
    *)
        echo "Error: Unknown RUN_MODE: $RUN_MODE"
        echo "Available modes: compat, native, maintenance"
        exit 1
        ;;
esac
