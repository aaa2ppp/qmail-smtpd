# Основной образ
FROM alpine:3.20


# Установка зависимостей для сборки и runtime
RUN apk add --no-cache \
    lang \
    build-base \
    curl \
    tar \
    patch \
    netcat-openbsd \
    tzdata \
    groff \
    ucspi-tcp6

RUN cd /tmp && \
    curl -sLO https://netqmail.org/netqmail-1.06.tar.gz && \
    echo "c922f776140b2c83043a6195901c67d3  netqmail-1.06.tar.gz" | md5sum -c - && \
    curl -sLO https://downloads.sourceforge.net/project/vpopmail/vpopmail-stable/5.4.33/vpopmail-5.4.33.tar.gz && \
    echo "0b5c8febc20f543d2ad835f3f789a9fc  vpopmail-5.4.33.tar.gz" | md5sum -c - && \
    true

# Установка netqmail (без sendmail-совместимости)
RUN addgroup -g 81 nofiles && \
    addgroup -g 83 qmail   && \
    adduser -u 81 -G nofiles -s /sbin/nologin -h /var/qmail/alias -D -H alias  && \
    adduser -u 82 -G nofiles -s /sbin/nologin -h /var/qmail -D -H qmaild && \
    adduser -u 83 -G nofiles -s /sbin/nologin -h /var/qmail -D -H qmaill && \
    adduser -u 84 -G nofiles -s /sbin/nologin -h /var/qmail -D -H qmailp && \
    adduser -u 85 -G qmail   -s /sbin/nologin -h /var/qmail -D -H qmailq && \
    adduser -u 86 -G qmail   -s /sbin/nologin -h /var/qmail -D -H qmailr && \
    adduser -u 87 -G qmail   -s /sbin/nologin -h /var/qmail -D -H qmails && \
    cd /tmp && \
    tar xzf netqmail-1.06.tar.gz && \
    cd netqmail-1.06 && \
    echo "gcc -O2" > conf-cc && \
    echo "gcc" > conf-ld && \
    make man && \
    make setup check && \
    mkdir -p /var/qmail/configure && \
    rm -rf /tmp/netqmail-1.06* && \
    true

# Установка vpopmail
RUN addgroup -g 89 vchkpw  && \
    adduser -u 89 -G vchkpw  -s /sbin/nologin -h /home/vpopmail -D vpopmail && \
    cd /tmp && \
    tar xzf vpopmail-5.4.33.tar.gz && \
    cd vpopmail-5.4.33 && \
    ./configure \   
        --enable-logging=p \
        --disable-passwd \
        --enable-clear-passwd \
        --enable-incdir=/var/qmail/include \
        --enable-libdir=/var/qmail/lib \
        --prefix=/home/vpopmail && \
    make && \
    make install-strip && \
    chmod ug+s /home/vpopmail/bin/vchkpw && \
    rm -rf /tmp/vpopmail-5.4.33* && \
    true

COPY ./bin/qmail-smtpd /usr/local/bin/my-qmail-smtpd
COPY ./entrypoint.sh /

EXPOSE 25

CMD ["/bin/sh", "/entrypoint.sh"]
