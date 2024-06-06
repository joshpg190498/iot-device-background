package models

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

type DeviceReadingSetting struct {
	IDDevice  string
	Parameter string
	Period    int
	Active    bool
}

type DeviceInfo struct {
	IDDevice string
	Field    string
	Value    string
}

type DeviceUpdate struct {
	IDDevice          string
	State             string
	UpdateDatetimeUTC string
}

type MessageConfigPayload struct {
	State    string
	Settings []DeviceReadingSetting
}

type ResponseConfigPayload struct {
	State             string
	SystemInfo        []DeviceInfo
	UpdateDatetimeUTC string
}

type DataPayload struct {
	IDDevice          string
	Parameter         string
	Data              any
	UpdateDatetimeUTC string
}
