package mqtt

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"project-tap/internal/pkg/database"
)

type MQTTService struct {
	Client mqtt.Client
	Store  database.Store
}

type ScanPayload struct {
	UID        string `json:"uid"`
	TerminalID string `json:"terminal_id"`
}

type FrontendPayload struct {
	CardNumber string `json:"card_number"`
	TerminalID string `json:"terminal_id"`
	Type       string `json:"type"`
}

// NewMQTTService initializes a new MQTT connection and sets up subscriptions
func NewMQTTService(store database.Store) (*MQTTService, error) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://127.0.0.1:1883" // fallback
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(fmt.Sprintf("unicard-backend-%d", time.Now().Unix()))
	opts.SetCleanSession(true)
	opts.OnConnect = func(client mqtt.Client) {
		log.Println("Connected to MQTT Broker!")
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("MQTT Connection Lost: %v", err)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	svc := &MQTTService{
		Client: client,
		Store:  store,
	}

	svc.subscribeTopics()

	return svc, nil
}

func (s *MQTTService) subscribeTopics() {
	topics := map[string]string{
		"unicard/retail/scan":    "Payment Store",
		"unicard/transport/scan": "Fare",
	}

	for topic, scanType := range topics {
		tType := scanType // capture for closure
		if token := s.Client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
			s.handleScanMessage(msg, tType)
		}); token.Wait() && token.Error() != nil {
			log.Printf("Error subscribing to %s: %v", topic, token.Error())
		} else {
			log.Printf("Subscribed to %s", topic)
		}
	}
}

func (s *MQTTService) handleScanMessage(msg mqtt.Message, scanType string) {
	var payload ScanPayload
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		log.Printf("Failed to unmarshal scan payload: %v", err)
		return
	}

	// Lookup card_number by UID
	var cardNumber string
	err := s.Store.QueryRow("SELECT card_number FROM cards WHERE card_uid = ?", payload.UID).Scan(&cardNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Card not found for UID: %s", payload.UID)
		} else {
			log.Printf("Database error querying UID %s: %v", payload.UID, err)
		}
		return
	}

	// Publish translated info for the frontend
	frontendPayload := FrontendPayload{
		CardNumber: cardNumber,
		TerminalID: payload.TerminalID,
		Type:       scanType,
	}

	b, _ := json.Marshal(frontendPayload)
	frontendTopic := "unicard/frontend/scan"
	if token := s.Client.Publish(frontendTopic, 0, false, b); token.Wait() && token.Error() != nil {
		log.Printf("Failed to publish to frontend topic: %v", token.Error())
	} else {
		log.Printf("Published %s to %s", string(b), frontendTopic)
	}
}
