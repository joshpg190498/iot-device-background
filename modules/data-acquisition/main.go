package main

import (
	"ceiot-tf-sbc/modules/data-acquisition/config"
	"ceiot-tf-sbc/modules/data-acquisition/mqtt"
	"ceiot-tf-sbc/modules/data-acquisition/sqlite"
	"encoding/json"
	"log"
	"sync"
	"time"
)

var (
	cfg          *config.Config
	settings     []sqlite.DeviceReadingSetting
	wg           sync.WaitGroup
	mutex        sync.Mutex // Mutex para proteger el acceso a `settings`
	stopChannels []chan struct{}
)

func main() {
	loadConfiguration()
	initializeDatabase()
	startMQTTClient()
	startDataAcquisition()
	select {} // Keep the main function running
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
	if topic == cfg.MQTTSubTopics[0] {
		log.Println("Received message for first topic, updating settings")
		newSettings := parseMessageToSettings(message)
		updateSettings(newSettings)
		log.Println(settings)
	}
}

func updateSettings(newSettings []sqlite.DeviceReadingSetting) {
	mutex.Lock()
	defer mutex.Unlock()

	// Detener las goroutines actuales
	for _, ch := range stopChannels {
		close(ch)
	}
	stopChannels = nil

	// Actualizar los settings
	for i := range settings {
		found := false
		for _, newSetting := range newSettings {
			if settings[i].Parameter == newSetting.Parameter {
				settings[i].Period = newSetting.Period
				settings[i].Active = newSetting.Active
				found = true
				break
			}
		}
		if !found {
			log.Printf("Parameter not found: %s\n", settings[i].Parameter)
		}
	}

	// Reiniciar las goroutines
	for index := range settings {
		wg.Add(1)
		stopChan := make(chan struct{})
		stopChannels = append(stopChannels, stopChan)
		go func(i int, stopChan chan struct{}) {
			defer wg.Done()
			runPeriodically(i, stopChan)
		}(index, stopChan)
	}
}

func parseMessageToSettings(message []byte) []sqlite.DeviceReadingSetting {
	var newSettings []sqlite.DeviceReadingSetting
	err := json.Unmarshal(message, &newSettings)
	if err != nil {
		log.Printf("Error parsing message: %v", err)
		return nil
	}
	return newSettings
}

func startDataAcquisition() {
	var err error
	settings, err = sqlite.GetDeviceReadingSettings()
	if err != nil {
		log.Fatalf("Error getting device reading settings: %v", err)
	}

	for index := range settings {
		wg.Add(1)
		stopChan := make(chan struct{})
		stopChannels = append(stopChannels, stopChan)
		go func(i int, stopChan chan struct{}) {
			defer wg.Done()
			runPeriodically(i, stopChan)
		}(index, stopChan)
	}
}

func runPeriodically(index int, stopChan chan struct{}) {
	for {
		mutex.Lock()
		if !settings[index].Active {
			mutex.Unlock()
			return
		}
		log.Printf("%s, %d\n", settings[index].Parameter, settings[index].Period)
		mutex.Unlock()

		timer := time.NewTimer(time.Duration(settings[index].Period) * time.Second)
		select {
		case <-stopChan:
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
