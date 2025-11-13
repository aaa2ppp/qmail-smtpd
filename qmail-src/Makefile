QMAIL     := qmail-1.03
NETQMAIL  := netqmail-1.06
UCSPI_TCP := ucspi-tcp-0.88
NAME      ?= $(NETQMAIL)
MD5       := ./md5.sh

all: unpack

.PHONY: clean all


.fetch-$(NAME):
	curl `cat $(NAME).tar.gz.url` -o $(NAME).tar.gz
	touch .fetch-$(NAME)

$(MD5):
	(echo '#!/bin/sh'; echo `which md5sum || echo md5 -r` '$$@') > $(MD5) && chmod +x $(MD5)

.check-$(NAME): .fetch-$(NAME) $(MD5)
	test `$(MD5) $(NAME).tar.gz | awk '{print $$1}'` = `cat $(NAME).tar.gz.md5 | awk '{print $$1}'`
	touch .check-$(NAME)

.unpack-$(NAME): .check-$(NAME)
	tar -xz -f $(NAME).tar.gz
	touch .unpack-$(NAME)

fetch: .fetch-$(NAME)

check: .check-$(NAME)

unpack: .unpack-$(NAME)

clean-all:
	rm -fr *.tar.gz $(QMAIL) $(NETQMAIL) .fetch-* .check-* .unpack-* $(MD5)

clean-src:
	rm -fr $(QMAIL) $(NETQMAIL) .unpack-*
