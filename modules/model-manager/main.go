// model-manager: agente del dispositivo que recibe órdenes de despliegue por
// MQTT (server/deploy/<id_device>), descarga el artefacto .tflite, verifica su
// sha256, lo activa por symlink y publica el estado en devices/<id_device>/deploy.
package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"ceiot-tf-sbc/internal/mqtt"
	"ceiot-tf-sbc/modules/model-manager/config"
	"ceiot-tf-sbc/modules/model-manager/deploy"
	"ceiot-tf-sbc/modules/model-manager/models"
)

// Cola de despliegues: el callback MQTT solo encola; un único worker procesa
// en orden. Así nunca hay dos descargas/activaciones concurrentes sobre el
// mismo modelo y el callback de paho no se bloquea durante una descarga larga.
const queueSize = 16

var (
	cfg   *models.Config
	queue = make(chan models.DeployCommand, queueSize)
)

func main() {
	var err error
	cfg, err = config.LoadEnvVars()
	if err != nil {
		log.Fatalf("Failed to load environment variables: %v", err)
	}
	log.Printf("model-manager iniciado: device=%s models_dir=%s certs_dir=%s sub=%v",
		cfg.IDDevice, cfg.ModelsDir, cfg.CertsDir, cfg.MQTTSubTopics)

	go worker()
	go mqtt.ConnectClient(cfg.MQTTBroker, cfg.MQTTClientID, cfg.MQTTSubTopics, cfg.CertsDir, handleMessage)
	select {}
}

func handleMessage(topic string, message []byte) {
	if topic != cfg.MQTTSubTopics[0] {
		return
	}
	var cmd models.DeployCommand
	if err := json.Unmarshal(message, &cmd); err != nil {
		log.Printf("Error parsing deploy command: %v", err)
		publish(models.DeployStatus{Status: models.StatusFailed, Message: "payload JSON inválido"})
		return
	}
	select {
	case queue <- cmd:
	default:
		log.Printf("Cola de despliegue llena; se descarta %s v%d", cmd.IDModel, cmd.Version)
		publish(statusFor(cmd, models.StatusFailed, false, "cola de despliegue llena"))
	}
}

func worker() {
	for cmd := range queue {
		process(cmd)
	}
}

func process(cmd models.DeployCommand) {
	log.Printf("Deploy recibido: model=%s version=%d activate=%v", cmd.IDModel, cmd.Version, cmd.Activate)

	if err := deploy.ValidateCommand(cmd.IDModel, cmd.Version, cmd.URL, cmd.SHA256); err != nil {
		log.Printf("Comando inválido: %v", err)
		publish(statusFor(cmd, models.StatusFailed, false, err.Error()))
		return
	}

	// Idempotencia 1: la versión pedida ya está activa → solo re-confirmar.
	if cur, err := deploy.CurrentVersion(cfg.ModelsDir, cmd.IDModel); err == nil &&
		cur == cmd.Version && cmd.Activate &&
		deploy.HasVerifiedVersion(cfg.ModelsDir, cmd.IDModel, cmd.Version, cmd.SHA256) {
		log.Printf("%s v%d ya está activa; se re-confirma", cmd.IDModel, cmd.Version)
		publish(statusFor(cmd, models.StatusActivated, true, "ya activa"))
		return
	}

	// Idempotencia 2 (rollback): la versión ya está en disco y verificada →
	// activar sin volver a descargar.
	if deploy.HasVerifiedVersion(cfg.ModelsDir, cmd.IDModel, cmd.Version, cmd.SHA256) {
		if !cmd.Activate {
			publish(statusFor(cmd, models.StatusDownloaded, true, "ya presente en disco"))
			return
		}
		if err := deploy.ActivateExisting(cfg.ModelsDir, cmd.IDModel, cmd.Version); err != nil {
			log.Printf("Error reactivando %s v%d desde disco: %v", cmd.IDModel, cmd.Version, err)
			publish(statusFor(cmd, models.StatusFailed, true, err.Error()))
			return
		}
		log.Printf("%s v%d reactivada desde disco (sin descargar)", cmd.IDModel, cmd.Version)
		publish(statusFor(cmd, models.StatusActivated, true, "reactivada desde disco"))
		return
	}

	// Camino normal: descarga + sha256 + (opcional) activación.
	res, err := deploy.DownloadVerifyActivate(
		cfg.ModelsDir, cmd.IDModel, cmd.Version,
		resolveURL(cmd.URL), cmd.SHA256, cmd.Activate, cfg.DownloadTimeout,
	)
	switch {
	case err != nil:
		log.Printf("Deploy fallido %s v%d: %v", cmd.IDModel, cmd.Version, err)
		publish(statusFor(cmd, models.StatusFailed, res.SHA256OK, err.Error()))
	case res.Activated:
		log.Printf("Deploy OK %s v%d activado", cmd.IDModel, cmd.Version)
		publish(statusFor(cmd, models.StatusActivated, true, ""))
	default:
		publish(statusFor(cmd, models.StatusDownloaded, true, ""))
	}
}

// resolveURL admite URLs absolutas o relativas a SERVER_HTTP_BASE, para que el
// servidor pueda mandar "/models/<id>/versions/<v>/model.tflite" sin conocer
// cómo lo ve la Pi (útil cuando el servidor está detrás de NAT o proxy).
func resolveURL(u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return cfg.ServerHTTPBase + "/" + strings.TrimLeft(u, "/")
}

func statusFor(cmd models.DeployCommand, status string, shaOK bool, msg string) models.DeployStatus {
	return models.DeployStatus{
		IDModel:        cmd.IDModel,
		IDModelVersion: cmd.IDModelVersion,
		Version:        cmd.Version,
		Status:         status,
		SHA256OK:       shaOK,
		Message:        msg,
	}
}

func publish(s models.DeployStatus) {
	s.IDDevice = cfg.IDDevice
	s.TimestampUTC = time.Now().UTC().Format(time.RFC3339)
	b, err := json.Marshal(s)
	if err != nil {
		log.Printf("Error converting to JSON: %v", err)
		return
	}
	mqtt.PublishData(cfg.MQTTPubDeployTopic, string(b))
}
