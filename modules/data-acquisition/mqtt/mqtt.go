package mqtt

import (
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	client         mqtt.Client
	opts           *mqtt.ClientOptions
	isConnected    bool
	connectionLock sync.Mutex
)

func ConnectClient(DeviceID string, MQTTBroker string, MQTTClientID string, MQTTSubTopics []string, handleMessage func(message string)) {

	if MQTTSubTopics == nil {
		MQTTSubTopics = []string{}
	}

	onMessageReceived := func(client mqtt.Client, message mqtt.Message) {
		handleMessage(string(message.Payload()))
	}

	onConnectionLost := func(client mqtt.Client, err error) {
		connectionLock.Lock()
		isConnected = false
		connectionLock.Unlock()
		fmt.Println("Conexión perdida:", err)
	}

	onConnect := func(client mqtt.Client) {
		connectionLock.Lock()
		isConnected = true
		connectionLock.Unlock()
		fmt.Printf("Conexión al broker %s con client-id %s\n", MQTTBroker, MQTTClientID)
		for _, MQTTSubTopic := range MQTTSubTopics {
			if token := client.Subscribe(MQTTSubTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
				fmt.Printf("Error al suscribirse a %s: %v\n", MQTTSubTopic, token.Error())
			} else {
				fmt.Printf("Suscrito al tópico %s\n", MQTTSubTopic)
			}
		}
	}

	opts = mqtt.NewClientOptions().
		AddBroker(MQTTBroker).
		SetClientID(MQTTClientID).
		SetConnectionLostHandler(onConnectionLost).
		SetOnConnectHandler(onConnect).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(5 * time.Second).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Error al conectar:", token.Error())
	}
}

func PublishData(topic string, data string) {
	connectionLock.Lock()
	defer connectionLock.Unlock()

	if client == nil || !isConnected {
		fmt.Println("El cliente MQTT no está conectado.")
		return
	}

	token := client.Publish(topic, 0, false, data)
	token.Wait()
	if token.Error() != nil {
		fmt.Printf("Error al publicar en el tópico %s: %v\n", topic, token.Error())
	} else {
		fmt.Printf("Mensaje publicado en el tópico %s: %s\n", topic, data)
	}
}
