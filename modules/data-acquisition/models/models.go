package models

type Config struct {
	IDDevice           string
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

type DeviceUpdate struct {
	IDDevice          string
	HashUpdate        string
	Type              string
	UpdateDatetimeUTC string
}

type MessageConfigPayload struct {
	IDDevice   string
	HashUpdate string
	Type       string
	Settings   []DeviceReadingSetting
}

type ResponseConfigPayload struct {
	IDDevice              string
	HashUpdate            string
	Type                  string
	MainDeviceInformation any
	UpdateDatetimeUTC     string
}

type DataPayload struct {
	IDDevice       string
	Parameter      string
	Data           any
	CollectedAtUtc string
}
