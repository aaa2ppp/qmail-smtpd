#!/bin/sh

# Скрипт для объединения исходных файлов проекта
# Использование: ./merge_code.sh <директория1> <директория2> ...

for dir in "$@"; do
    # Ищем файлы с нужными расширениями исключая пути
    find "$dir" -type f     \
        ! -path './qmail-src/*'      \
        ! -path '*/.*'      \
        ! -path '*/tmp/*'   \
        ! -path '*/bak/*'   \
        ! -path '*/bak[0-9]/*' \
        ! -path '*/bin/*'   \
        ! -path '*/data/*'  \
        ! -path '*/migrations/test/*' \
        \( \
        -name 'LICENSE*'    \
        -o -name '*.go'     \
        -o -name '*.sql'    \
        -o -name '*.js'     \
        -o -name '*.sh'     \
        -o -name '*.md'     \
        -o -name 'Dockerfile*' \
        -o -name '*.yml'    \
        -o -name '*.yaml'   \
        -o -name 'Makefile*'\
        -o -name '*.example'\
        \) |
    while read -r f; do
        # Убираем ./ в начале пути
        f="${f#./}"
        
        # Выводим содержимое с заголовком
        echo "== $f =="
        echo
        cat "$f"
        echo
    done
done
