#!/bin/bash
set -euo pipefail

export CGO_ENABLED="${CGO_ENABLED:-1}"
export GOOS="${GOOS:-linux}"
export GOARCH="${GOARCH:-amd64}"

if [ "${CGO_ENABLED}" != "1" ]; then
    echo "Ошибка: проект использует github.com/mattn/go-sqlite3, ему нужен CGO_ENABLED=1" >&2
    exit 1
fi

rm -f bot

go build -ldflags="-s -w" -o bot .
echo "Скомпилировано успешно (CGO_ENABLED=${CGO_ENABLED})"

if command -v upx >/dev/null; then
    upx --best bot
    echo "Бинарный файл успешно сжат"
else
    echo "upx не найден, пропускаю сжатие"
fi
