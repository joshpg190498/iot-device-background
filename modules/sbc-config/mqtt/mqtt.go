package mqtt

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func ConnectClient(mqttHost string, mqttPort string, mqttClientID string, mqttSubTopic string, handleMessage func(message string)) {

	var client mqtt.Client
	var token mqtt.Token
	var opts *mqtt.ClientOptions

	connect := func(opts *mqtt.ClientOptions) {
		client = mqtt.NewClient(opts)
		if token = client.Connect(); token.Wait() && token.Error() != nil {
			fmt.Println("Error al reconectar:", token.Error())
		}
	}

	onMessageReceived := func(client mqtt.Client, message mqtt.Message) {
		handleMessage(string(message.Payload()))
	}

	onConnectionLost := func(client mqtt.Client, err error) {
		fmt.Println("Perdió conexión")
	}

	onConnect := func(client mqtt.Client) {
		if token = client.Subscribe(mqttSubTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
			fmt.Printf("Error al suscribirse a %s: %v\n", mqttSubTopic, token.Error())
		} else {
			fmt.Printf("Suscrito a %s\n", mqttSubTopic)
		}
	}

	opts = mqtt.NewClientOptions()
	opts.AddBroker("tcp://" + mqttHost + ":" + mqttPort)
	opts.SetClientID(mqttClientID)
	opts.SetAutoReconnect(true)
	opts.SetConnectionLostHandler(onConnectionLost)
	opts.SetConnectRetry(true)
	opts.OnConnect = onConnect

	connect(opts)
}
