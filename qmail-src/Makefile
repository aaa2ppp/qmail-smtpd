NAME=qmail-1.03
MD5=./.md5

all: unpack

.PHONY: clean all

.fetch:
	curl `cat $(NAME).tar.gz.url` -o $(NAME).tar.gz && touch .fetch

.md5:
	(echo '#!/bin/sh'; echo `which md5sum || echo md5 -r` '$$@') > $(MD5) && chmod +x $(MD5)

.check: .fetch .md5
	test `$(MD5) $(NAME).tar.gz | awk '{print $$1}'` = `cat $(NAME).tar.gz.md5 | awk '{print $$1}'` && touch .check

.unpack: .check
	tar -xz -f $(NAME).tar.gz && touch .unpack

fetch: .fetch

check: .check

unpack: .unpack

clean:
	rm -fr qmail-1.03.tar.gz qmail-1.03 .fetch .check .unpack .md5
