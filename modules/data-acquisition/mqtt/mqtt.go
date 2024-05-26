package mqtt

import (
	"log"
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

func ConnectClient(DeviceID string, MQTTBroker string, MQTTClientID string, MQTTSubTopics []string, handleMessage func(topic string, message []byte)) {

	if MQTTSubTopics == nil {
		MQTTSubTopics = []string{}
	}

	onMessageReceived := func(client mqtt.Client, message mqtt.Message) {
		handleMessage(message.Topic(), message.Payload())
	}

	onConnectionLost := func(client mqtt.Client, err error) {
		connectionLock.Lock()
		isConnected = false
		connectionLock.Unlock()
		log.Println("Conexión perdida:", err)
	}

	onConnect := func(client mqtt.Client) {
		connectionLock.Lock()
		isConnected = true
		connectionLock.Unlock()
		log.Printf("Conexión al broker %s con client-id %s\n", MQTTBroker, MQTTClientID)
		for _, MQTTSubTopic := range MQTTSubTopics {
			if token := client.Subscribe(MQTTSubTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
				log.Printf("Error al suscribirse a %s: %v\n", MQTTSubTopic, token.Error())
			} else {
				log.Printf("Suscrito al tópico %s\n", MQTTSubTopic)
			}
		}
	}

	opts = mqtt.NewClientOptions().
		AddBroker(MQTTBroker).
		SetClientID(MQTTClientID).
		SetConnectionLostHandler(onConnectionLost).
		SetOnConnectHandler(onConnect).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(2 * time.Second).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second)

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Println("Error al conectar:", token.Error())
	}
}

func PublishData(topic string, data string) {
	connectionLock.Lock()
	defer connectionLock.Unlock()

	if client == nil || !isConnected {
		log.Println("El cliente MQTT no está conectado.")
		return
	}

	token := client.Publish(topic, 0, false, data)
	token.Wait()
	if token.Error() != nil {
		log.Printf("Error al publicar en el tópico %s: %v\n", topic, token.Error())
	} else {
		log.Printf("Mensaje publicado en el tópico %s: %s\n", topic, data)
	}
}
