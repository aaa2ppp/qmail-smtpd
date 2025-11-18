AUTO_QMAIL    ?= ./var/qmail

BIN_DIR ?= $(AUTO_QMAIL)/bin
TMP_DIR ?= ./tmp

GOEXE := $(shell go env GOEXE)
TEST_FLAGS ?=
TEST_FLAGS := $(TEST_FLAGS) -tags=dev

MERGE_FILES ?= Makefile go.mod go.sum *.go *.sh *.md

# source and destination for merge/patch operations
SRC ?= .
DST ?= 1

# Кастомные флаги сборки (можно переопределить при вызове make)
GO_BUILD_FLAGS ?=

# Находим все поддиректории в cmd, которые потенциально могут быть бинарниками
CMDS := $(wildcard cmd/*)

# Генерируем список целей для бинарников
BINARIES := $(patsubst cmd/%,$(BIN_DIR)/%,$(CMDS))


.PHONY: FORCE all deps test build clean

FORCE:


# Основная цель - собирает все бинарники
all: deps test build

# Правило для подготовки зависимостей
deps:
	go mod tidy

test:
	go test $(TEST_FLAGS) ./...
	@echo OK


# Шаблонное правило для сборки любого бинарника
$(BIN_DIR)/%: FORCE
	@mkdir -p $(@D)
	go build $(GO_BUILD_FLAGS) -o $@$(GOEXE) ./cmd/$(notdir $@)

build: $(BINARIES)

# Очистка
clean:
	-rm -rf $(BIN_DIR) $(TMP_DIR)


.PHONY: merge patch

MERGE_FIND_PARTS := $(patsubst %,-o -name '%',$(MERGE_FILES))
MERGE_FIND_EXPR := $(wordlist 2,$(words $(MERGE_FIND_PARTS)),$(MERGE_FIND_PARTS))

merge:
	@mkdir -p $(TMP_DIR)
	@find $(SRC) -type f \( $(MERGE_FIND_EXPR) \) -exec sh -c 'name="{}"; printf "== $${name#./} ==\n\n"; cat $$name; echo' ';' > $(TMP_DIR)/$(DST).code
	@echo "Merge saved to $(TMP_DIR)/$(DST).code"	
	

# Создает прекоммит патч
patch: test
	@mkdir -p $(TMP_DIR)
	
	@(set -e; \
	staged_list="$(TMP_DIR)/staged_list.$$$$"; \
	unstaged_list="$(TMP_DIR)/unstaged_list.$$$$"; \
	git diff --staged --name-only -- $(SRC) > "$$staged_list"; \
	git diff --name-only -- $(SRC) > "$$unstaged_list"; \
	intersection=$$(grep -Fxf "$$staged_list" "$$unstaged_list" || true); \
	rm -f "$$staged_list" "$$unstaged_list"; \
	if [ -n "$$intersection" ]; then \
		echo "" >&2; \
		echo "WARNING: the following files have changes not staged for commit:" >&2; \
		echo "  (use \"git add <file>...\" to update what will be committed)" >&2; \
		printf '%s\n' $$intersection | sed 's/^/        /' >&2; \
		echo "" >&2; \
	fi)
	
	git diff --staged -- $(SRC) > $(TMP_DIR)/$(DST).patch
	@echo "Patch saved to $(TMP_DIR)/$(DST).patch"


.PHONY: qmail-tree qmail-test1 qmail-test-tls qmail-test-tcprules

qmail-tree:
	mkdir -p $(AUTO_QMAIL)
	cd $(AUTO_QMAIL)
	mkdir bin tmp control

qmail-test1: $(AUTO_QMAIL)/bin/addcr $(AUTO_QMAIL)/bin/qmail-smtpd
	AUTO_QMAIL=$(AUTO_QMAIL) sh ./tests/test1/test.sh

qmail-test-tls: $(AUTO_QMAIL)/bin/safeout
	AUTO_QMAIL=$(AUTO_QMAIL) go run ./tests/tls 2>&1 | $(AUTO_QMAIL)/bin/safeout

qmail-test-tcprules: $(AUTO_QMAIL)/bin/tcprules
	AUTO_QMAIL=$(AUTO_QMAIL) sh ./tests/tcprules/test.sh

qmail-test-env:
	sh ./tests/env/test.sh
