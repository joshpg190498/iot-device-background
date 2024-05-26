// models.go
package sqlite

// Estructura para almacenar los datos de DEVICE_READING_SETTINGS
type DeviceReadingSetting struct {
	IDDevice  string
	Parameter string
	Period    int
	Active    bool
}

// Estructura para almacenar los datos de DEVICE
type Device struct {
	IDDevice string
	Field    string
	Value    string
}

// Estructura para almacenar los datos de DEVICE_UPDATES
type DeviceUpdate struct {
	IDDevice          string
	UpdateDatetimeUTC string
}
