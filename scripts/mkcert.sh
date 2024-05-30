#! /bin/sh

name=servercert
user=

if ! which openssl >/dev/null 2>/dev/null; then
	echo "openssl not found" >&2
fi

if [ "x$AUTO_QMAIL" == x ]; then
	echo "env AUTO_QMAIL required"
fi

cd "$AUTO_QMAIL/control" || exit 1

if test -f "$name.pem"; then
	echo "$name.pem already exists."
	exit 1
fi

umask 077
set -e

cleanup() {
	rm -f "$name.pem"
	rm -f "$name.rand"
	rm -f "$name.key"
	rm -f "$name.cert"
	exit 1
}

touch "$name.pem"
chmod 600 "$name.pem"
if [ "x$user" != x ]; then
	chown "$user:$user" "$name.pem"
fi

dd if=/dev/urandom of="$name.rand" count=1 # WTF? count=1
#To use the certificate request to create a self-signed certificate for testing purposes, type the following command:
#openssl x509 -req -in <certificat request> -extfile myssl.cnf -extensions req_ext -signkey <ssl key> -days <number of days> -out <certificate name>
openssl req -new -x509 -days 365 -nodes \
	-config "$name.cnf" -extensions req_ext -out "$name.pem" -keyout "$name.pem" || cleanup
rm -f "$name.rand"

openssl x509 -subject -dates -fingerprint -noout -in "$name.pem" || cleanup
