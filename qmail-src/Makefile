NAME=qmail-1.03

.PHONY: clean

.fetch:
	curl `cat $(NAME).tar.gz.url` -o $(NAME).tar.gz && touch .fetch

.check: .fetch
	test `md5sum $(NAME).tar.gz | awk '{print $$1}'` = `cat $(NAME).tar.gz.md5 | awk '{print $$1}'` && touch .check

.unpack: .check
	tar -xz -f qmail-1.03.tar.gz && touch .unpack

fetch: .fetch

check: .check

unpack: .unpack

clean:
	rm -fr qmail-1.03.tar.gz qmail-1.03 .fetch .check .unpack
