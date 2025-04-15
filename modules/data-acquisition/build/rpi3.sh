
#!/bin/bash

echo "=== Building rpi3 ==="

sudo apt install gcc-arm-linux-gnueabihf -y

echo "** GCC compatible instalado correctamente **"

export CC=arm-linux-gnueabihf-gcc
export GOOS=linux
export GOARCH=arm
export GOARM=7
export CGO_ENABLED=1

go build -o v1.0.0-raspberry

echo "** Compilado generado correctamente **"
