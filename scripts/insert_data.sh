#!/bin/bash

# Запуск: ./scripts/insert_data.sh
DB_FILE="data/pens.db"
CSV_FILE="scripts/data.csv"

# Функция для вставки или обновления данных в таблице
insert_or_update() {
    local pen_name=$1
    local pen_length=$2
    local handsome_count=$3
    local unhandsome_count=$4

    echo "Processing: $pen_name, $pen_length, $handsome_count, $unhandsome_count"

    # Проверка, существует ли пользователь в таблице
    user_exists=$(sqlite3 $DB_FILE "SELECT COUNT(*) FROM pens WHERE pen_name='$pen_name';")
    echo "User exists: $user_exists"

    if [ "$user_exists" -eq "1" ]; then
        # Обновление данных пользователя
        sqlite3 $DB_FILE "UPDATE pens SET pen_length=$pen_length, handsome_count=$handsome_count, unhandsome_count=$unhandsome_count WHERE pen_name='$pen_name';"
        echo "Updated: $pen_name"
    else
        # Вставка нового пользователя
        echo "Not found, push /pen: $pen_name"
    fi
}

# Чтение данных из CSV-файла и вызов функции insert_or_update
while IFS=';' read -r pen_name pen_length handsome_count unhandsome_count; do
    if [ -n "$pen_name" ]; then
        insert_or_update "$pen_name" "$pen_length" "$handsome_count" "$unhandsome_count"
    fi
done < $CSV_FILE