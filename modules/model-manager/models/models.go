// Package models define la configuración del model-manager y los tipos del
// contrato de despliegue (deploy) que viajan por MQTT entre el servidor y el
// dispositivo.
package models

import "time"

// Config son los parámetros de ejecución del model-manager (cargados de .env).
type Config struct {
	IDDevice           string
	MQTTHost           string
	MQTTPort           string
	MQTTClientID       string
	MQTTBroker         string
	MQTTSubTopics      []string // [0] = server/deploy/<id_device>
	MQTTPubDeployTopic string   // devices/<id_device>/deploy (ACK de estado)
	CertsDir           string   // directorio con ca-crt.pem, client-crt.pem, client-key.pem
	ModelsDir          string   // raíz ABSOLUTA de artefactos: $MODELS_DIR/<id_model>/...
	ServerHTTPBase     string   // opcional: prefijo para URLs relativas ("/models/..")
	DownloadTimeout    time.Duration
}

// DeployCommand es la orden que baja del servidor por server/deploy/<id_device>.
type DeployCommand struct {
	IDModel        string `json:"id_model"`
	IDModelVersion int    `json:"id_model_version"`
	Version        int    `json:"version"`
	URL            string `json:"url"`    // absoluta, o relativa a SERVER_HTTP_BASE
	SHA256         string `json:"sha256"` // hex, 64 caracteres
	Activate       bool   `json:"activate"`
}

// Estados posibles del ACK de despliegue.
const (
	StatusDownloaded = "downloaded" // descargado y verificado, sin activar
	StatusActivated  = "activated"  // verificado y 'current' apunta a esta versión
	StatusFailed     = "failed"     // no se activó nada (ver Message)
)

// DeployStatus es el ACK que sube el dispositivo por devices/<id_device>/deploy.
type DeployStatus struct {
	IDDevice       string `json:"id_device"`
	IDModel        string `json:"id_model"`
	IDModelVersion int    `json:"id_model_version"`
	Version        int    `json:"version"`
	Status         string `json:"status"`
	SHA256OK       bool   `json:"sha256_ok"`
	Message        string `json:"message"`
	TimestampUTC   string `json:"timestamp_utc"`
}
