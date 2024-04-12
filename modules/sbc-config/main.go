package main

import (
	"fmt"
	"os"
	"os/signal"
	config "sbc-config/config"
	mqtt "sbc-config/mqtt"
	"syscall"
)

func handleMessage(message string) {
	fmt.Printf("Message: %s\n", message)
}

func main() {
	config.LoadEndVars()
	for {
		mqtt.ConnectClient(config.MQTTHost, config.MQTTPort, config.MQTTClientID, config.MQTTSubTopic, handleMessage)

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("Exiting...")
		os.Exit(0)
	}
}
