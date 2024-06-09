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
	go mqtt.ConnectClient(cfg.DeviceID, cfg.MQTTBroker, cfg.MQTTClientID, cfg.MQTTSubTopics, handleMessage)
}

func handleMessage(topic string, message []byte) {
	if topic != cfg.MQTTSubTopics[0] {
		return
	}
	messagePayload := parseMessageToSettings(message)

	var responseConfigPayload models.ResponseConfigPayload

	responseConfigPayload, err := handleUpdate(messagePayload)
	if err != nil {
		return
	}

	jsonData, err := json.Marshal(responseConfigPayload)
	if err != nil {
		log.Printf("Error converting to JSON: %s", err)
		return
	}

	mqtt.PublishData(cfg.MQTTPubConfigTopic, string(jsonData))
}

func parseMessageToSettings(message []byte) models.MessageConfigPayload {
	var messageConfigPayload models.MessageConfigPayload
	if err := json.Unmarshal(message, &messageConfigPayload); err != nil {
		log.Printf("Error parsing message: %v", err)
	}
	return messageConfigPayload
}

func handleUpdate(messagePayload models.MessageConfigPayload) (models.ResponseConfigPayload, error) {
	var responseConfigPayload models.ResponseConfigPayload
	responseConfigPayload.Type = messagePayload.Type
	responseConfigPayload.DeviceID = messagePayload.DeviceID
	responseConfigPayload.HashUpdate = messagePayload.HashUpdate

	if responseConfigPayload.Type != "startup" && responseConfigPayload.Type != "update" {
		return models.ResponseConfigPayload{}, fmt.Errorf("received message for unknown state: %s", messagePayload.Type)
	}

	deviceUpdate, err := sqlite.GetDeviceUpdates(messagePayload.Type, messagePayload.HashUpdate)
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
	err = sqlite.InsertMainDeviceInformation(cfg.DeviceID, mainDeviceInformation)
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

			err = publishData(dataPayload)
			if err != nil {
				log.Println("Error converting to JSON:", err)
			}

			mutex.Unlock()
			period := time.Duration(settings[index].Period) * time.Second
			timer.Reset(period)
		}
	}
}

func collectData(index int) (models.DataPayload, error) {
	var dataPayload models.DataPayload
	utcTime := time.Now().UTC()
	formattedTime := utcTime.Format(time.RFC3339)
	data, err := system.CallFunctionByName(settings[index].Parameter)
	if err != nil {
		return dataPayload, err
	}

	dataPayload.DeviceID = cfg.DeviceID
	dataPayload.Parameter = settings[index].Parameter
	dataPayload.Data = data
	dataPayload.CollectedAtUtc = formattedTime
	return dataPayload, nil
}

func publishData(dataPayload models.DataPayload) error {
	jsonData, err := json.Marshal(dataPayload)
	if err != nil {
		return err
	}

	mqtt.PublishData(cfg.MQTTPubDataTopic, string(jsonData))
	return nil
}
