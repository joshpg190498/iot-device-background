package main

import (
	"ceiot-tf-sbc/modules/data-acquisition/config"
	"ceiot-tf-sbc/modules/data-acquisition/models"
	"ceiot-tf-sbc/modules/data-acquisition/mqtt"
	"ceiot-tf-sbc/modules/data-acquisition/sqlite"
	"ceiot-tf-sbc/modules/data-acquisition/system"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	cfg          *models.Config
	settings     []models.DeviceReadingSetting
	wg           sync.WaitGroup
	mutex        sync.Mutex
	stopChannels []chan struct{}
)

func main() {
	loadConfiguration()
	startMQTTClient()
	initializeDatabase()
	time.AfterFunc(3*time.Second, startDataAcquisition)
	select {}
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
	go mqtt.ConnectClient(cfg.MQTTBroker, cfg.MQTTClientID, cfg.MQTTSubTopics, handleMessage)
}

func handleMessage(topic string, message []byte) {
	var err error
	if topic != cfg.MQTTSubTopics[0] {
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
		return "", nil
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
	mainDeviceInformation, err := sqlite.GetMainDeviceInformation()
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
			mutex.Lock()
			if !settings[index].Active {
				mutex.Unlock()
				continue
			}

			dataPayload, err := collectData(index)
			if err != nil {
				log.Println("Error:", err)
				mutex.Unlock()
				continue
			}

			mqttPayload, err := stringifyPayload(dataPayload)
			if err == nil {
				mqtt.PublishData(cfg.MQTTPubDataTopic, mqttPayload)
			}

			mutex.Unlock()
			period := time.Duration(settings[index].Period) * time.Second
			timer.Reset(period)
		}
	}
}

func collectData(index int) (models.DataPayload, error) {
	dataPayload := models.DataPayload{}
	utcTime := time.Now().UTC()
	formattedTime := utcTime.Format(time.RFC3339)
	data, err := system.CallFunctionByName(settings[index].Parameter)
	if err != nil {
		return dataPayload, err
	}

	dataPayload.IDDevice = cfg.IDDevice
	dataPayload.Parameter = settings[index].Parameter
	dataPayload.Data = data
	dataPayload.CollectedAtUtc = formattedTime
	return dataPayload, nil
}
