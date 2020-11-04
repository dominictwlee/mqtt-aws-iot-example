package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"
)

type HRData struct {
	Event string `json:"event"`
	Data  int    `json:"data"`
}

type Payload struct {
	Message HRData `json:"message"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("Error loading .env file")
	}

	tlsConfig, err := newTLSConfig()
	if err != nil {
		log.Fatalf("Failed to create TLS config: %v", err)
	}

	opts := mqtt.NewClientOptions()
	cid := "someThing"
	host := os.Getenv("HOST")
	port := 8883
	brokerURL := fmt.Sprintf("tcps://%s:%d", host, port)

	opts.AddBroker(brokerURL)
	opts.SetClientID(cid)
	opts.SetTLSConfig(tlsConfig)
	opts.SetCleanSession(true)
	opts.SetDefaultPublishHandler(msgHandler)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("failed to connect: %v", token.Error())
	}
	log.Println("[MQTT] Connected")

	go func() {
		time.Sleep(time.Second * 3)
		payload := Payload{
			Message: HRData{
				Event: "heartrate",
				Data:  80,
			},
		}
		data, err := json.Marshal(&payload)
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		if token := client.Publish("/dummy", 0, false, data); token.Wait() && token.Error() != nil {
			log.Fatalf("failed to publish to /dummy: %v", token.Error())
		}
	}()

	if token := client.Subscribe("/dummy", 0, nil); token.Wait() && token.Error() != nil {
		log.Fatalf("failed to create subscription: %v", token.Error())
	}
	fmt.Println("Subscribed to /dummy")

	done := make(chan bool)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	go func() {
		<-sig
		client.Disconnect(250)
		fmt.Println("[MQTT] Disconnected")
		done <- true
	}()
	<-done
}

var msgHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("TOPIC: %s\n", msg.Topic())
	fmt.Printf("MSG: %s\n", msg.Payload())
}

func newTLSConfig() (*tls.Config, error) {
	certPool := x509.NewCertPool()
	pemCerts, err := ioutil.ReadFile(os.Getenv("ROOT_PEM"))
	if err != nil {
		return nil, err
	}
	certPool.AppendCertsFromPEM(pemCerts)

	cert, err := tls.LoadX509KeyPair(os.Getenv("PUB_CERT"), os.Getenv("PRIV_KEY"))
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		RootCAs:      certPool,
		ClientAuth:   tls.NoClientCert,
		Certificates: []tls.Certificate{cert},
	}
	return config, nil
}
