package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Variables globales para las configuraciones
var (
	DeviceID     string
	MQTTHost     string
	MQTTPort     string
	MQTTClientID string
	MQTTSubTopic string
)

func LoadEndVars() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error al obtener el directorio actual:", err)
		return
	}

	parentDir := filepath.Join(dir, "..", "..")

	envFile := filepath.Join(parentDir, ".env")

	err = godotenv.Load(envFile)
	if err != nil {
		fmt.Println("Error al cargar el archivo .env:", err)
		return
	}

	DeviceID = os.Getenv("ID_DEVICE")
	MQTTHost = os.Getenv("MQTT_HOST")
	MQTTPort = os.Getenv("MQTT_PORT")
	MQTTClientID = "mqtt-sbc-config-" + DeviceID
	MQTTSubTopic = "CONFIG/" + DeviceID
}
