#!/bin/bash

# Запуск: ./scripts/build.sh
# Установите переменные окружения для статической компиляции
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64

# Скомпилируйте приложение с уменьшением размера бинарного файла
if go build -ldflags="-s -w" -o bot; then
    echo "Скомпилировано успешно"
    # Сжимаем бинарный файл с помощью upx
    if upx --best bot; then
        echo "Бинарный файл успешно сжат"
    else
        echo "Ошибка сжатия бинарного файла"
    fi
else
    echo "Ошибка компиляции"
fi