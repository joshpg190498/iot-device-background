package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ceiot-tf-sbc/modules/model-manager/models"

	"github.com/joho/godotenv"
)

const defaultDownloadTimeoutSec = 120

// LoadEnvVars carga el .env ubicado junto al ejecutable (mismo patrón que
// data-acquisition) y arma la configuración del model-manager.
func LoadEnvVars() (*models.Config, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("error al obtener el directorio actual: %w", err)
	}
	exeDir := filepath.Dir(exe)
	if err := godotenv.Load(filepath.Join(exeDir, ".env")); err != nil {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	idDevice := os.Getenv("ID_DEVICE")
	if idDevice == "" {
		return nil, fmt.Errorf("ID_DEVICE no definido en .env")
	}

	mqttProtocol := os.Getenv("MQTT_PROTOCOL")
	mqttHost := os.Getenv("MQTT_HOST")
	mqttPort := os.Getenv("MQTT_PORT")

	// MODELS_DIR: obligatorio. Si es relativo, se resuelve contra el directorio
	// del ejecutable (systemd no garantiza el cwd que uno espera).
	modelsDir := os.Getenv("MODELS_DIR")
	if modelsDir == "" {
		return nil, fmt.Errorf("MODELS_DIR no definido en .env")
	}
	if !filepath.IsAbs(modelsDir) {
		modelsDir = filepath.Join(exeDir, modelsDir)
	}
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creando MODELS_DIR %s: %w", modelsDir, err)
	}

	// CERTS_DIR: opcional. Por defecto certs/ junto al ejecutable (mismo layout
	// que data-acquisition en build/bin-files/certs). Relativo → contra exeDir.
	certsDir := os.Getenv("CERTS_DIR")
	if certsDir == "" {
		certsDir = filepath.Join(exeDir, "certs")
	} else if !filepath.IsAbs(certsDir) {
		certsDir = filepath.Join(exeDir, certsDir)
	}

	timeoutSec := defaultDownloadTimeoutSec
	if v := os.Getenv("DOWNLOAD_TIMEOUT_SEC"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("DOWNLOAD_TIMEOUT_SEC inválido: %q", v)
		}
		timeoutSec = n
	}

	cfg := &models.Config{
		IDDevice:           idDevice,
		MQTTHost:           mqttHost,
		MQTTPort:           mqttPort,
		MQTTClientID:       fmt.Sprintf("background-model-manager-%s-mqtt-client", idDevice),
		MQTTBroker:         fmt.Sprintf("%s://%s:%s", mqttProtocol, mqttHost, mqttPort),
		MQTTSubTopics:      []string{fmt.Sprintf("server/deploy/%s", idDevice)},
		MQTTPubDeployTopic: fmt.Sprintf("devices/%s/deploy", idDevice),
		CertsDir:           certsDir,
		ModelsDir:          modelsDir,
		ServerHTTPBase:     strings.TrimRight(os.Getenv("SERVER_HTTP_BASE"), "/"),
		DownloadTimeout:    time.Duration(timeoutSec) * time.Second,
	}
	return cfg, nil
}
