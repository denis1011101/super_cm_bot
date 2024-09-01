#!/bin/bash

# Запуск: ./scripts/build.sh
# Установите переменные окружения для статической компиляции
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# Скомпилируйте приложение с уменьшением размера бинарного файла
go build -ldflags="-s -w" -o bot