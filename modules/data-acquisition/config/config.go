package config

import (
	"fmt"
	"os"

	"ceiot-tf-sbc/modules/data-acquisition/models"
)

func LoadEnvVars() (*models.Config, error) {
	idDevice := os.Getenv("ID_DEVICE")
	mqttProtocol := os.Getenv("MQTT_PROTOCOL")
	mqttHost := os.Getenv("MQTT_HOST")
	mqttPort := os.Getenv("MQTT_PORT")

	mqttClientID := fmt.Sprintf("background-data-acquisition-%s-mqtt-client", idDevice)
	mqttBroker := fmt.Sprintf("%s://%s:%s", mqttProtocol, mqttHost, mqttPort)

	mqttSubConfigTopic := fmt.Sprintf("server/config/%s", idDevice)
	mqttSubTopics := []string{mqttSubConfigTopic}

	mqttPubConfigTopic := fmt.Sprintf("devices/%s/config", idDevice)
	mqttPubDataTopic := fmt.Sprintf("devices/%s/data", idDevice)
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
