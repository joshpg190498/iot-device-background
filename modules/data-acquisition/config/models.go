package config

// Config estructura para las configuraciones
type Config struct {
	DeviceID           string
	MQTTHost           string
	MQTTPort           string
	MQTTClientID       string
	MQTTBroker         string
	MQTTSubTopics      []string
	MQTTPubConfigTopic string
	MQTTPubDataTopic   string
	DatabasePath       string
}
