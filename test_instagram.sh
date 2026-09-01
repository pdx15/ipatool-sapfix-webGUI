#!/bin/bash

# Тестовый скрипт для проверки скачивания Instagram
# ID приложения: 389801252
# Bundle ID: com.burbn.instagram

echo "=== Тест скачивания Instagram ==="
echo ""

# Собираем проект
echo "1. Сборка проекта..."
cd /home/user/ipatool-sapfix-webGUI
go build -o ipatool-gui .

if [ $? -ne 0 ]; then
    echo "Ошибка сборки!"
    exit 1
fi

echo "✓ Сборка успешна"
echo ""

# Запускаем GUI
echo "2. Запуск GUI..."
echo "Откройте браузер и проверьте:"
echo "   - Поиск Instagram"
echo "   - Попытку скачивания"
echo "   - Ошибку (если будет)"
echo ""
echo "Запуск GUI с verbose логами..."
./ipatool-gui gui --verbose
