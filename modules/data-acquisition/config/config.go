package config

import (
	"fmt"
	"os"
	"path/filepath"

	"ceiot-tf-sbc/modules/data-acquisition/models"

	"github.com/joho/godotenv"
)

func LoadEnvVars() (*models.Config, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error al obtener el directorio actual: %w", err)
	}

	parentDir := filepath.Join(dir, "..", "..")
	envFile := filepath.Join(parentDir, ".env")

	err = godotenv.Load(envFile)
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	idDevice := os.Getenv("ID_DEVICE")
	mqttProtocol := os.Getenv("MQTT_PROTOCOL")
	mqttHost := os.Getenv("MQTT_HOST")
	mqttPort := os.Getenv("MQTT_PORT")

	mqttClientID := fmt.Sprintf("mqtt-sbc-data-acquisition-%s", idDevice)
	mqttBroker := fmt.Sprintf("%s://%s:%s", mqttProtocol, mqttHost, mqttPort)

	mqttSubConfigTopic := fmt.Sprintf("SERVER/CONFIG/%s", idDevice)
	mqttSubTopics := []string{mqttSubConfigTopic}

	mqttPubConfigTopic := fmt.Sprintf("DEVICES/%s/CONFIG", idDevice)
	mqttPubDataTopic := fmt.Sprintf("DEVICES/%s/DATA", idDevice)
	databasePath := os.Getenv("SQLITE_DB_PATH")

	config := &models.Config{
		IDDevice:           idDevice,
		MQTTHost:           mqttHost,
		MQTTPort:           mqttPort,
		MQTTClientID:       mqttClientID,
		MQTTBroker:         mqttBroker,
		MQTTSubTopics:      mqttSubTopics,
		MQTTPubConfigTopic: mqttPubConfigTopic,
		MQTTPubDataTopic:   mqttPubDataTopic,
		DatabasePath:       databasePath,
	}

	return config, nil
}
