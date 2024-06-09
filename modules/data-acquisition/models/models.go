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
	DeviceID  string
	Parameter string
	Period    int
	Active    bool
}

type DeviceUpdate struct {
	DeviceID          string
	HashUpdate        string
	Type              string
	UpdateDatetimeUTC string
}

type MessageConfigPayload struct {
	DeviceID   string
	HashUpdate string
	Type       string
	Settings   []DeviceReadingSetting
}

type ResponseConfigPayload struct {
	DeviceID              string
	HashUpdate            string
	Type                  string
	MainDeviceInformation any
	UpdateDatetimeUTC     string
}

type DataPayload struct {
	DeviceID       string
	Parameter      string
	Data           any
	CollectedAtUtc string
}
