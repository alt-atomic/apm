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
CONFIG_SRC="$REPO_DIR/data/cat/apm/config.yml"
if [ ! -f "$CONFIG_SRC" ]; then
  echo "Файл конфигурации $CONFIG_SRC не найден!"
  exit 1
fi
echo "Копирование конфигурационного файла в /etc/apm/config.yml..."
cp "$CONFIG_SRC" /etc/apm/config.yml || { echo "Не удалось скопировать конфигурационный файл!"; exit 1; }


PO_FILE="$REPO_DIR/po/ru.po"
if [ ! -f "$PO_FILE" ]; then
  echo "Файл локализации $PO_FILE не найден!"
  exit 1
fi

# Копирование D‑Bus‑конфига
DBUS_CONF_SRC="$REPO_DIR/data/dbus-config/apm.conf"
DBUS_CONF_DST="/etc/dbus-1/system.d/apm.conf"

if [ ! -f "$DBUS_CONF_SRC" ]; then
  echo "Файл D‑Bus‑конфигурации $DBUS_CONF_SRC не найден!"
  exit 1
fi

echo "Копирование D‑Bus‑конфигурации в $DBUS_CONF_DST..."
install -o root -g root -m 644 "$DBUS_CONF_SRC" "$DBUS_CONF_DST" \
  || { echo "Не удалось установить D‑Bus‑конфигурацию!"; exit 1; }

# Перезапуск системного dbus‑daemon, чтобы он перечитал конфиг
echo "Перезапуск systemd‑dbus.service..."
systemctl try-reload-or-restart dbus.service

# Удаление файла базы данных, если он существует
if [ -f "/var/apm/apm.db" ]; then
  echo "Удаление файла базы данных /var/apm/apm.db..."
  rm -f /var/apm/apm.db || { echo "Не удалось удалить файл /var/apm/apm.db"; exit 1; }
fi

echo "Копирование файла локализации ru.po в /usr/share/locales..."
cp "$PO_FILE" /usr/share/locale/ru/LC_MESSAGES/apm.po || { echo "Не удалось скопировать файл ru.po!"; exit 1; }

# Копирование бинарного файла в /usr/bin/apm
echo "Копирование бинарного файла в /usr/bin/apm..."
cp "$REPO_DIR/apm" /usr/bin/apm || { echo "Не удалось переместить бинарный файл в /usr/bin/apm"; exit 1; }

echo "Установка завершена. Для справки выполните: apm -help"
