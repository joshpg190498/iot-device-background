#!/usr/bin/env bash

# Detener el script si ocurre algún error
set -e

# Define el directorio de salida
OUT_DIR="prod"

echo "Iniciando proceso de build para producción..."

# 1. Crear (o limpiar) la carpeta prod
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

# 2. Copiar el archivo .env a la carpeta prod
if [ -f ".env" ]; then
    cp .env "$OUT_DIR/.env"
    echo "Archivo .env copiado a $OUT_DIR/"
else
    echo "Advertencia: No se encontró el archivo .env en la raíz."
fi

# 3. Recorrer cada subcarpeta en 'modules' y compilar si existe un main.go
if [ -d "modules" ]; then
    for dir in modules/*/; do
        # Verificar si existe main.go dentro del módulo
        if [ -f "${dir}main.go" ]; then
            # Obtener solo el nombre del módulo (ej: modules/auth/ -> auth)
            module_name=$(basename "$dir")
            
            echo "Compilando módulo: $module_name..."
            
            # Compilar el binario optimizado para producción (-ldflags "-s -w" reduce el tamaño)
            go build -ldflags "-s -w" -o "$OUT_DIR/$module_name" "./$dir"
            
            echo "   ↳ Generado: $OUT_DIR/$module_name"
        fi
    done
else
    echo "Error: La carpeta 'modules' no existe."
    exit 1
fi

echo "Build completado con éxito."