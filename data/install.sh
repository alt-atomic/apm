#!/bin/bash

# Проверка, что скрипт запущен с правами суперпользователя
if [ "$EUID" -ne 0 ]; then
  echo "Пожалуйста, запустите скрипт с правами суперпользователя (sudo)."
  exit 1
fi

if [ -f "/usr/bin/bootc" ]; then
  echo "Файл /usr/bin/bootc найден. Выполнение команды: bootc usr-overlay..."
  bootc usr-overlay
fi

# Проверка наличия Go
if ! command -v go &> /dev/null; then
  apt-get install -y go
fi

# Проверка наличия Git
if ! command -v git &> /dev/null; then
  apt-get install -y git
fi

REPO_URL="https://github.com/alt-atomic/apm"
REPO_DIR="/tmp/apm"

if [ ! -d "$REPO_DIR" ]; then
  echo "Клонирование репозитория APM в $REPO_DIR..."
  git clone "$REPO_URL" "$REPO_DIR" || { echo "Ошибка клонирования репозитория!"; exit 1; }
else
  echo "Обновление репозитория APM в $REPO_DIR..."
  cd "$REPO_DIR" || { echo "Не удалось перейти в директорию $REPO_DIR"; exit 1; }
  git pull || { echo "Ошибка обновления репозитория!"; exit 1; }
fi

cd "$REPO_DIR" || { echo "Не удалось перейти в директорию $REPO_DIR"; exit 1; }

echo "Сборка APM..."
go build -o apm || { echo "Сборка завершилась неудачно!"; exit 1; }

# Создание и настройка системных директорий
echo "Создание и настройка системных директорий..."
mkdir -p /etc/apm
mkdir -p /var/apm
chmod 777 /var/apm

# Копирование конфигурационного файла
CONFIG_SRC="$REPO_DIR/data/config.yml"
if [ ! -f "$CONFIG_SRC" ]; then
  echo "Файл конфигурации $CONFIG_SRC не найден!"
  exit 1
fi
echo "Копирование конфигурационного файла в /etc/apm/config.yml..."
cp "$CONFIG_SRC" /etc/apm/config.yml || { echo "Не удалось скопировать конфигурационный файл!"; exit 1; }

# Копирование переводов (локалей)
LOCALES_SRC="$REPO_DIR/data/locales"
if [ ! -d "$LOCALES_SRC" ]; then
  echo "Папка с переводами $LOCALES_SRC не найдена!"
  exit 1
fi

echo "Копирование папки локалей из $LOCALES_SRC в /var/apm/locales..."
rm -rf /var/apm/locales
cp -r "$LOCALES_SRC" /var/apm/locales || { echo "Не удалось скопировать папку локалей!"; exit 1; }

# Копирование бинарного файла в /usr/bin/apm
echo "Копирование бинарного файла в /usr/bin/apm..."
cp "$REPO_DIR/apm" /usr/bin/apm || { echo "Не удалось переместить бинарный файл в /usr/bin/apm"; exit 1; }

echo "Установка завершена. Для справки выполните: apm -help"
