package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {
	// Obtener el directorio actual
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error al obtener el directorio actual:", err)
		return
	}

	// Retroceder un directorio
	parentDir := filepath.Join(dir, "..")

	// Construir la ruta completa al archivo .local.env
	envFile := filepath.Join(parentDir, ".env")

	// Cargar las variables de entorno desde el archivo .env
	err = godotenv.Load(envFile)
	if err != nil {
		fmt.Println("Error al cargar el archivo .local.env:", err)
		return
	}

	// Ahora puedes acceder a las variables de entorno cargadas
	// Ejemplo de uso:
	fmt.Println("Valor de la variable de entorno ID_DEVICE:", os.Getenv("ID_DEVICE"))
}
