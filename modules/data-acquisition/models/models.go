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
	IDDevice  string //	Identificador del dispositivo
	Parameter string //	Identificador del parámetro
	Period    int    // Periodo de lectura del parámetro
	Active    bool   // Estado del parámetro
}

type MessageConfigPayload struct {
	IDDevice   string                 // Identificador del dispositivo
	HashUpdate string                 // Identificador del mensaje
	Type       string                 // Tipo de actualización
	Settings   []DeviceReadingSetting // Configuraciones
}

type DeviceUpdate struct {
	IDDevice          string
	HashUpdate        string
	Type              string
	UpdateDatetimeUTC string
}

type ResponseConfigPayload struct {
	IDDevice              string // Identificador del dispositivo
	HashUpdate            string // Identificador del mensaje
	Type                  string // Tipo de actualización
	MainDeviceInformation any    // Información principal del dispositivo
	UpdateDatetimeUTC     string // Fecha de actualización del dispositivo
}

type DataPayload struct {
	IDDevice       string      // Identificador del dispositivo
	Parameter      string      // Identificador del parámetro
	Data           interface{} // Información del parámetro medido
	CollectedAtUtc string      // Fecha de adquisición de la data, UTC.
}
