package main

import (
	"ceiot-tf-sbc/modules/data-acquisition/config"
	"ceiot-tf-sbc/modules/data-acquisition/mqtt"
	"ceiot-tf-sbc/modules/data-acquisition/sqlite"

	"fmt"
)

func handleMessage(message string) {
	fmt.Printf("Message: %s\n", message)
}

func main() {
	//system.GetSystemInfo()
	config.LoadEndVars()
	go forever()
	//go publishData()
	select {}
}

func forever() {
	err := sqlite.InitDB(config.DatabasePath)
	if err != nil {
		mqtt.ConnectClient(config.DeviceID, config.MQTTBroker, config.MQTTClientID, config.MQTTSubTopics, handleMessage)
	}
}
