
#!/bin/bash

echo "=== Building rpi4 ==="

sudo apt install gcc-aarch64-linux-gnu -y

echo "** GCC compatible instalado correctamente **"

export CC=aarch64-linux-gnu-gcc
export GOOS=linux
export GOARCH=arm64
export CGO_ENABLED=1

go build -o v1.0.0-raspberry

echo "** Compilado generado correctamente **"