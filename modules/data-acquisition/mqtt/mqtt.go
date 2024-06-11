package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

func ConnectClient(MQTTBroker string, MQTTClientID string, MQTTSubTopics []string, handleMessage func(topic string, message []byte)) {

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

	caFile, certFile, keyFile, err := getCertPaths()
	if err != nil {
		log.Fatalf("Error encontrando certificados: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Fatalf("Error cargando certificado CA: %v", err)
	}
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("Error cargando certificado y clave: %v", err)
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
		ClientCAs:          nil,
		ClientAuth:         tls.NoClientCert,
	}

	opts = mqtt.NewClientOptions().
		AddBroker(MQTTBroker).
		SetClientID(MQTTClientID).
		SetConnectionLostHandler(onConnectionLost).
		SetOnConnectHandler(onConnect).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(2 * time.Second).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second).
		SetTLSConfig(tlsConfig)

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

func getCertPaths() (caPath, clientCertPath, clientKeyPath string, err error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", "", "", err
	}

	// Retroceder un nivel para acceder a la carpeta certs
	certsDir := filepath.Join(dir, "..", "..", "certs")

	caPath = filepath.Join(certsDir, "ca-crt.pem")
	clientCertPath = filepath.Join(certsDir, "client-crt.pem")
	clientKeyPath = filepath.Join(certsDir, "client-key.pem")

	if _, err := os.Stat(caPath); err != nil {
		return "", "", "", fmt.Errorf("error: CA certificate file not found: %v", err)
	}
	if _, err := os.Stat(clientCertPath); err != nil {
		return "", "", "", fmt.Errorf("error: client certificate file not found: %v", err)
	}
	if _, err := os.Stat(clientKeyPath); err != nil {
		return "", "", "", fmt.Errorf("error: client key file not found: %v", err)
	}

	return caPath, clientCertPath, clientKeyPath, nil
}
