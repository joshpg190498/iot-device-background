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

	if messagePayload.State == "initialization" {
		responseConfigPayload := handleInitialization(messagePayload)

		jsonData, err := json.Marshal(responseConfigPayload)
		if err != nil {
			log.Fatalf("Error converting to JSON: %s", err)
		}

		fmt.Println(string(jsonData))

		mqtt.PublishData(cfg.MQTTPubConfigTopic, string(jsonData))
	} else if messagePayload.State == "updating" {
		log.Println("Received message for updating state, updating settings")

		responseConfigPayload := handleUpdating(messagePayload)

		jsonData, err := json.Marshal(responseConfigPayload)
		if err != nil {
			log.Fatalf("Error converting to JSON: %s", err)
		}

		fmt.Println(string(jsonData))

		mqtt.PublishData(cfg.MQTTPubConfigTopic, string(jsonData))
	} else {
		log.Printf("Received message for unknown state: %s", messagePayload.State)
	}
}

func parseMessageToSettings(message []byte) models.MessageConfigPayload {
	var messageConfigPayload models.MessageConfigPayload
	if err := json.Unmarshal(message, &messageConfigPayload); err != nil {
		log.Printf("Error parsing message: %v", err)
		return models.MessageConfigPayload{}
	}
	return messageConfigPayload
}

func handleInitialization(messagePayload models.MessageConfigPayload) models.ResponseConfigPayload {
	var responseConfigPayload models.ResponseConfigPayload
	responseConfigPayload.State = messagePayload.State

	deviceUpdate, err := sqlite.GetDeviceUpdates(messagePayload.State)
	if err != nil {
		log.Printf("Error getting DeviceUpdates: %v", err)
		return responseConfigPayload
	}

	if len(deviceUpdate) >= 1 {
		responseConfigPayload.SystemInfo = getDeviceInfo()
		responseConfigPayload.UpdateDatetimeUTC = deviceUpdate[0].UpdateDatetimeUTC
	} else {
		responseConfigPayload.SystemInfo = insertAndGetDeviceInfo()
		updateSettings(messagePayload.Settings)
		utcTime, err := sqlite.UpdateSettings(messagePayload.State, messagePayload.Settings)
		if err != nil {
			log.Printf("Error inserting new settings: %v", err)
		} else {
			responseConfigPayload.UpdateDatetimeUTC = utcTime.Format(time.RFC3339)
		}
	}

	return responseConfigPayload
}

func handleUpdating(messagePayload models.MessageConfigPayload) models.ResponseConfigPayload {
	var responseConfigPayload models.ResponseConfigPayload
	updateSettings(messagePayload.Settings)
	utcTime, err := sqlite.UpdateSettings(messagePayload.State, messagePayload.Settings)
	if err != nil {
		log.Printf("Error inserting new settings: %v", err)
	} else {
		responseConfigPayload.UpdateDatetimeUTC = utcTime.Format(time.RFC3339)
	}
	return responseConfigPayload
}

func getDeviceInfo() []models.DeviceInfo {
	deviceInfo, err := sqlite.GetDeviceInfoFields()
	if err != nil {
		log.Printf("Error getting DeviceInfo: %v", err)
		return nil
	}
	return deviceInfo
}

func insertAndGetDeviceInfo() []models.DeviceInfo {
	deviceInfo, err := system.GetDeviceInfo(cfg.DeviceID)
	if err != nil {
		log.Printf("Error getting DeviceInfo: %v", err)
		return nil
	}
	err = sqlite.InsertDeviceInfoFields(deviceInfo)
	if err != nil {
		log.Printf("Error inserting DeviceInfo: %v", err)
		return nil
	}
	return deviceInfo
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

	dataPayload.IDDevice = cfg.DeviceID
	dataPayload.Parameter = settings[index].Parameter
	dataPayload.Data = data
	dataPayload.UpdateDatetimeUTC = formattedTime
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
