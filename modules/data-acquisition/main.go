package main

import (
	"ceiot-tf-sbc/modules/sbc-data-acquisition/config"
	"ceiot-tf-sbc/modules/sbc-data-acquisition/mqtt"
	"ceiot-tf-sbc/modules/sbc-data-acquisition/system"

	"fmt"
	"time"
)

func handleMessage(message string) {
	fmt.Printf("Message: %s\n", message)
}

func main() {
	system.GetSystemInfo()
	config.LoadEndVars()
	go forever()
	go publishData()
	select {}
}

func forever() {
	mqtt.ConnectClient(config.DeviceID, config.MQTTBroker, config.MQTTClientID, config.MQTTSubTopics, handleMessage)
}

func publishData() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		mqtt.PublishData("topic", "bye")
	}
}
