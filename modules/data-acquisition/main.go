package main

import (
	"ceiot-tf-sbc/internal/mqtt"
	"ceiot-tf-sbc/modules/data-acquisition/config"
	"ceiot-tf-sbc/modules/data-acquisition/models"
	"ceiot-tf-sbc/modules/data-acquisition/sqlite"
	"ceiot-tf-sbc/modules/data-acquisition/system"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const mqttConnectTimeout = 10 * time.Second

const pendingMetricsFlushBatch = 50

var (
	cfg          *models.Config
	settings     []models.DeviceReadingSetting
	wg           sync.WaitGroup
	mutex        sync.Mutex
	stopChannels []chan struct{}
)

func main() {
	loadConfiguration()
	initializeDatabase()

	mqtt.SetOnConnectCallback(flushPendingMetrics)
	startMQTTClient()
	waitForConnectionOrTimeout(mqttConnectTimeout)

	startDataAcquisition()

	waitForShutdown()
}

func loadConfiguration() {
	var err error
	cfg, err = config.LoadEnvVars()
	if err != nil {
		log.Fatalf("Failed to load environment variables: %v", err)
	}
}

func initializeDatabase() {
	err := sqlite.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
}

func startMQTTClient() {
	go mqtt.ConnectClient(cfg.MQTTBroker, cfg.MQTTClientID, cfg.MQTTSubTopics, cfg.CertsDir, handleMessage)
}

func waitForConnectionOrTimeout(timeout time.Duration) {
	select {
	case <-mqtt.Connected:
		log.Println("Conectado al broker MQTT. Iniciando adquisición de datos.")
	case <-time.After(timeout):
		log.Printf("No se logró conectar al broker MQTT en %s. Iniciando adquisición en modo offline (buffer local).\n", timeout)
	}
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Señal de apagado recibida, deteniendo procesos de adquisición...")
	mutex.Lock()
	stopCurrentGoroutines()
	mutex.Unlock()

	wg.Wait()

	if err := sqlite.CloseDB(); err != nil {
		log.Println("Error cerrando base de datos:", err)
	}
	log.Println("Apagado completo.")
}

func handleMessage(topic string, message []byte) {
	var err error
	if len(cfg.MQTTSubTopics) == 0 || topic != cfg.MQTTSubTopics[0] {
		return
	}

	messagePayload, err := parseMessageToSettings(message)
	if err != nil {
		return
	}

	responseConfigPayload, err := handleUpdate(messagePayload)
	if err != nil {
		return
	}

	mqttPayload, err := stringifyPayload(responseConfigPayload)
	if err != nil {
		return
	}

	mqtt.PublishData(cfg.MQTTPubConfigTopic, mqttPayload)
}

func stringifyPayload(payload any) (string, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error converting to JSON: %s", err)
		return "", err
	}
	stringJsonData := string(jsonData)
	return stringJsonData, nil
}

func parseMessageToSettings(message []byte) (models.MessageConfigPayload, error) {
	messageConfigPayload := models.MessageConfigPayload{}
	if err := json.Unmarshal(message, &messageConfigPayload); err != nil {
		log.Printf("Error parsing message: %v", err)
		return models.MessageConfigPayload{}, err
	}
	return messageConfigPayload, nil
}

func handleUpdate(messagePayload models.MessageConfigPayload) (models.ResponseConfigPayload, error) {
	responseConfigPayload := models.ResponseConfigPayload{
		Type:       messagePayload.Type,
		IDDevice:   messagePayload.IDDevice,
		HashUpdate: messagePayload.HashUpdate,
	}

	if responseConfigPayload.Type != "startup" && responseConfigPayload.Type != "update" {
		return models.ResponseConfigPayload{}, fmt.Errorf("received message for unknown state: %s", messagePayload.Type)
	}

	if responseConfigPayload.Type == "startup" {
		log.Printf("Se recibió un mensaje de inicialización...")

	}

	if responseConfigPayload.Type == "update" {
		log.Printf("Se recibió un mensaje de actualización...")
	}

	deviceUpdate, err := sqlite.GetDeviceUpdates(cfg.IDDevice, messagePayload.Type, messagePayload.HashUpdate)
	if err != nil {
		log.Printf("Error getting DeviceUpdates: %v", err)
		return models.ResponseConfigPayload{}, err
	}

	if len(deviceUpdate) >= 1 {
		if messagePayload.Type == "startup" {
			responseConfigPayload.MainDeviceInformation = getMainDeviceInformation()
		}
		responseConfigPayload.UpdateDatetimeUTC = deviceUpdate[0].UpdateDatetimeUTC
	} else {
		if messagePayload.Type == "startup" {
			responseConfigPayload.MainDeviceInformation = getAndInsertDeviceInfo()
		}
		updateAndLogSettings(messagePayload, &responseConfigPayload)
	}

	if responseConfigPayload.UpdateDatetimeUTC == "" {
		return models.ResponseConfigPayload{}, fmt.Errorf("internal error while system updates configuration")
	}

	return responseConfigPayload, nil
}

func updateAndLogSettings(messagePayload models.MessageConfigPayload, responseConfigPayload *models.ResponseConfigPayload) {
	updateSettings(messagePayload.Settings)
	utcTime, err := sqlite.UpdateSettings(messagePayload.HashUpdate, messagePayload.Type, messagePayload.Settings)
	if err != nil {
		log.Printf("Error inserting new settings: %v", err)
	} else {
		responseConfigPayload.UpdateDatetimeUTC = utcTime.Format(time.RFC3339)
	}
}

func getMainDeviceInformation() map[string]interface{} {
	mainDeviceInformation, err := sqlite.GetMainDeviceInformation(cfg.IDDevice)
	if err != nil {
		log.Printf("Error getting MainDeviceInformation: %v", err)
		return nil
	}
	return mainDeviceInformation
}

func getAndInsertDeviceInfo() map[string]interface{} {
	mainDeviceInformation, err := system.CallFunctionByName("main_info")
	if err != nil {
		log.Printf("Error getting MainDeviceInformation: %v", err)
		return nil
	}
	err = sqlite.InsertMainDeviceInformation(cfg.IDDevice, mainDeviceInformation)
	if err != nil {
		log.Printf("Error inserting MainDeviceInformation: %v", err)
		return nil
	}
	return mainDeviceInformation
}

func updateSettings(newSettings []models.DeviceReadingSetting) {
	mutex.Lock()
	defer mutex.Unlock()

	stopCurrentGoroutines()
	updateDeviceSettings(newSettings)
	startNewGoroutines()
}

func stopCurrentGoroutines() {
	for _, ch := range stopChannels {
		close(ch)
	}
	stopChannels = nil
}

func updateDeviceSettings(newSettings []models.DeviceReadingSetting) {
	existingSettings := make(map[string]models.DeviceReadingSetting)
	for i := range settings {
		existingSettings[settings[i].Parameter] = settings[i]
	}

	for _, newSetting := range newSettings {
		existingSettings[newSetting.Parameter] = newSetting
	}

	settings = nil
	for _, setting := range existingSettings {
		settings = append(settings, setting)
	}
}

func startNewGoroutines() {
	for i := range settings {
		stopChan := make(chan struct{})
		stopChannels = append(stopChannels, stopChan)
		wg.Add(1)
		go func(index int, stopChan chan struct{}) {
			defer wg.Done()
			log.Printf("Inicializando proceso de adquisición para el parámetro %s", settings[i].Parameter)
			runPeriodically(index, stopChan)
		}(i, stopChan)
	}
}

func startDataAcquisition() {
	var err error
	settings, err = sqlite.GetDeviceReadingSettings()
	if err != nil {
		log.Fatalf("Error getting device reading settings: %v", err)
	}
	startNewGoroutines()
}

func runPeriodically(index int, stopChan chan struct{}) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-timer.C:
			period := processReading(index)
			timer.Reset(period)
		}
	}
}

func processReading(index int) time.Duration {
	parameter, active, period, ok := readSetting(index)
	if !ok {
		// El índice dejó de ser válido (la configuración cambió). Esta
		// goroutine debería recibir la señal de stopChan en breve; se
		// reintenta con un período corto por defecto mientras tanto.
		return time.Second
	}

	if !active {
		return period
	}

	dataPayload, err := collectData(cfg.IDDevice, parameter)
	if err != nil {
		log.Printf("Error recolectando el parámetro %s: %v\n", parameter, err)
		return period
	}

	mqttPayload, err := stringifyPayload(dataPayload)
	if err != nil {
		log.Printf("Error serializando payload de %s: %v\n", parameter, err)
		return period
	}

	publishOrBuffer(cfg.MQTTPubDataTopic, mqttPayload)

	return period
}

func readSetting(index int) (parameter string, active bool, period time.Duration, ok bool) {
	mutex.Lock()
	defer mutex.Unlock()

	if index < 0 || index >= len(settings) {
		return "", false, 0, false
	}
	s := settings[index]
	return s.Parameter, s.Active, time.Duration(s.Period) * time.Second, true
}

func publishOrBuffer(topic string, payload string) {
	if mqtt.IsConnected() && mqtt.PublishData(topic, payload) {
		return
	}
	if err := sqlite.InsertPendingMetric(topic, payload); err != nil {
		log.Println("Error bufferizando métrica pendiente:", err)
	}
}

func flushPendingMetrics() {
	for {
		pending, err := sqlite.GetPendingMetrics(pendingMetricsFlushBatch)
		if err != nil {
			log.Println("Error leyendo métricas pendientes:", err)
			return
		}
		if len(pending) == 0 {
			return
		}

		sentAny := false
		for _, p := range pending {
			if !mqtt.IsConnected() {
				return
			}
			if !mqtt.PublishData(p.Topic, p.Payload) {
				return
			}
			if err := sqlite.DeletePendingMetric(p.ID); err != nil {
				log.Println("Error eliminando métrica pendiente ya enviada:", err)
				return
			}
			sentAny = true
		}

		if !sentAny {
			return
		}
	}
}

func collectData(idDevice string, parameter string) (models.DataPayload, error) {
	dataPayload := models.DataPayload{}
	utcTime := time.Now().UTC()
	formattedTime := utcTime.Format(time.RFC3339)
	data, err := system.CallFunctionByName(parameter)
	if err != nil {
		return dataPayload, err
	}

	dataPayload.IDDevice = idDevice
	dataPayload.Parameter = parameter
	dataPayload.Data = data
	dataPayload.CollectedAtUtc = formattedTime
	return dataPayload, nil
}
