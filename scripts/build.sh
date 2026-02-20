#!/bin/bash
set -euo pipefail

export CGO_ENABLED=0
export GOOS="${GOOS:-linux}"
export GOARCH="${GOARCH:-amd64}"

rm -f bot

go build -ldflags="-s -w" -o bot .
echo "Скомпилировано успешно"

if command -v upx >/dev/null; then
    upx --best bot
    echo "Бинарный файл успешно сжат"
else
    echo "upx не найден, пропускаю сжатие"
fi
