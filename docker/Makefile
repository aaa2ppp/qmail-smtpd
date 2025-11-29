NAME   := myqmail
CMD    := ../cmd
BIN    := ./bin
TMP    := ./tmp
ENV    := ./.env
GOOS   := linux
GOARCH := amd64

.build: $(BIN)/qmail-smtpd
	docker build -t $(NAME) .
	@touch .build

.create: .build
	docker create \
		--name $(NAME) \
		--memory=50m \
		--memory-swap=50m \
		--memory-reservation=50m \
		--cpus=0.5 \
		-p 2525:25 \
		--env-file $(ENV) \
		$(NAME)
	@touch .create

$(BIN)/qmail-smtpd:
	mkdir -p $(TMP) $(BIN)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BIN)/qmail-smtpd $(CMD)/qmail-smtpd

.PHONY: build rm start stop clean

build: .build bin/qmail-smtpd

rm:
	docker rm $(NAME) && rm .create

start: .create
	docker start $(NAME)

stop:
	docker stop $(NAME)

clean:
	-docker stop $(NAME) && docker rm $(NAME) && rm .create
	-docker image rm $(NAME) && rm .build
	-rm -fr $(BIN) $(TMP)

stats:
	@x=$$(docker stats \
		--no-stream \
		--format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}") && \
	echo "$$x" | head -1 && \
	echo "$$x" | grep -e ^$(NAME)
