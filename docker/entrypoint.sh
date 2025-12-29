#!/bin/sh

# set -x
set -e

: ${LOCAL:?}

qmail_smtpd=${QMAIL_SMTPD:-"/usr/local/bin/my-qmail-smtpd"}
if ! test -e "$qmail_smtpd"; then
    echo "$qmail_smtpd: executable not found" >&2
    exit 1
fi

qmail=${AUTO_QMAIL:-~qmaild}
if [ "$qmail" = "~qmaild" ]; then
    echo "qmaild home has not been found, please define AUTO_QMAIL env" >&2
    exit 1
fi
if ! test -d "$qmail/bin"; then
    echo "$qmail/bin: directory not found" >&2
    exit 1
fi

vpopmail=${VPOPMAIL:-~vpopmail}
if [ "$vpopmail" = "~vpopmail" ]; then
    echo "vpopmail home has not been found, please define VPOPMAIL env" >&2
    exit 1
fi
if ! test -d "$vpopmail/bin"; then
    echo "$vpopmail/bin: directory not found" >&2
    exit 1
fi

tcpserver=$(which tcpserver)
if [ -z "$tcpserver" ] || [ ! -e "$tcpserver" ]; then
    tcpserver="/usr/local/bin/tcpserver"
fi
if [ -z "$tcpserver" ] || [ ! -e "$tcpserver" ]; then
    tcpserver="/usr/bin/tcpserver"
fi

qmaild_uid=$(id -u qmaild)
qmaild_gid=$(id -g qmaild)

# На первом запуске (нет домена $LOCAL в vpopmail) настраиваем qmail+vpopmail
if ! "$vpopmail/bin/vdominfo" "$LOCAL" >/dev/null 2>&1; then
    # удаляем конфигурацию инсталлятора
    rm -fr "$qmail/control" || true

    # создаем свою конфигурацию
    mkdir "$qmail/control"
    echo  "$LOCAL" > "$qmail/control/me"
    echo  "$LOCAL" > "$qmail/control/defaultdomain"
    echo  "$LOCAL" > "$qmail/control/plusdomain"
    # пустой locals, иначе будет использоваться me (vadddomain $LOCAL сам все настроит)
    touch "$qmail/control/locals"
    # пустрой rcpthosts, иначе будет открытый релей
    touch "$qmail/control/rcpthosts"

    # заворачиваем доставку на $LOCAL в vpopmail
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
        echo "Running tcpserver + original /var/qmail/bin/qmail-smtpd" >&2
        if [ -z "$tcpserver" ]; then
            echo "tcpserver has not been found, only "native" and "maintenance" run modes are available." >&2
            exit 1
        fi
        "$qmail/bin/qmail-start" ./Maildir/ &
        qmail_pid=$!
        "$tcpserver" -v -R -H -l "$LOCAL" -u "$qmaild_uid" -g "$qmaild_gid" 0 25 \
            "$qmail/bin/qmail-smtpd" \
            "$LOCAL" "$vpopmail/bin/vchkpw" true &
        smtpd_pid=$!
        wait
        ;;
    compat)
        echo "Running in qmail-compatible mode (with tcpserver)" >&2
        if [ -z "$tcpserver" ]; then
            echo "tcpserver has not been found, only "native" and "maintenance" run modes are available." >&2
            exit 1
        fi
        "$qmail/bin/qmail-start" ./Maildir/ &
        qmail_pid=$!
        "$tcpserver" -v -R -H -l "$LOCAL" -u "$qmaild_uid" -g "$qmaild_gid" 0 25 \
            "$qmail_smtpd" \
            "$LOCAL" "$vpopmail/bin/vchkpw" true &
        smtpd_pid=$!
        wait
        ;;
    native)
        echo "Running in native server mode" >&2
        "$qmail/bin/qmail-start" ./Maildir/ &
        qmail_pid=$!
        "$qmail_smtpd" --addr=:25 -l "$LOCAL" \
            "$vpopmail/bin/vchkpw" true &
        smtpd_pid=$!
        wait
        ;;
    maintenance)
        echo "Running in maintenance mode" >&2
        while true; do sleep 3600; done
        ;;
    *)
        echo "Error: Unknown RUN_MODE: $RUN_MODE" >&2
        echo "Available modes: original, compat, native, maintenance" >&2
        exit 1
        ;;
esac
