#!/bin/bash

set -e

echo "=== Script de despliegue a producción SBC PMS - Data Acquisition ==="

echo "** Variables de entorno **"

read -p "SQLITE_DB_PATH: " SQLITE_DB_PATH
read -p "MQTT_PROTOCOL: " MQTT_PROTOCOL
read -p "MQTT_HOST: " MQTT_HOST
read -p "MQTT_PORT: " MQTT_PORT
read -p "ID_DEVICE: " ID_DEVICE

cp .env.example .env

sed -i "s|__SQLITE_DB_PATH__|$SQLITE_DB_PATH|g" .env
sed -i "s|__MQTT_PROTOCOL__|$MQTT_PROTOCOL|g" .env
sed -i "s|__MQTT_HOST__|$MQTT_HOST|g" .env 
sed -i "s|__MQTT_PORT__|$MQTT_PORT|g" .env
sed -i "s|__ID_DEVICE__|$ID_DEVICE|g" .env

echo "Archivo .env configurado :)"

echo "** Modificar permisos y archivos necesarios **"

chmod 777 rpi4-v1.0.0

PWD_PATH=$(pwd)
sed -i "s|__Working_Directory__|$PWD_PATH|g" sbc-pms_data-acquisition.service

echo "Archios modificados correctamente :)"

echo "** Habilitar servicio **"
sudo cp sbc-pms_data-acquisition.service /etc/systemd/system
sudo systemctl daemon-reexec
sudo systemctl daemon-reload
sudo systemctl enable sbc-pms_data-acquisition.service
sudo systemctl start sbc-pms_data-acquisition.service

sudo journalctl -u sbc-pms_data-acquisition.service -n 20 --no-pager

echo "Servicio habilitado"

echo "Para revisar los logs ejecutar: journalctl -u sbc-pms_data-acquisition -f"

echo "=== Despliegue completo ==="
