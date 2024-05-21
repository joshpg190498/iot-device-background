package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Variables globales para las configuraciones
var (
	DeviceID           string
	MQTTHost           string
	MQTTPort           string
	MQTTClientID       string
	MQTTBroker         string
	MQTTSubTopics      []string
	MQTTSubConfigTopic string
	MQTTPubConfigTopic string
	MQTTPubDataTopic   string
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
	MQTTClientID = "mqtt-sbc-data-acquisition-" + DeviceID
	MQTTBroker = "tcp://" + MQTTHost + ":" + MQTTPort

	MQTTSubTopics = []string{}
	MQTTSubConfigTopic = fmt.Sprintf("SERVER/DEVICES/%s/CONFIG", DeviceID)
	MQTTSubTopics = append(MQTTSubTopics, MQTTSubConfigTopic)

	MQTTPubConfigTopic = fmt.Sprintf("DEVICES/%s/CONFIG", DeviceID)
	MQTTPubDataTopic = fmt.Sprintf("DEVICES/%s/DATA", DeviceID)

}
