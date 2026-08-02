package main

import (
	"log"

	"github.com/rxzsu/goldenpay-go"
)

type MyWebhookHandler struct{}

func (h *MyWebhookHandler) HandleWebhook(event goldenpay.WebhookEvent) error {
	switch event.Type {
	case goldenpay.EventNewOrder:
		log.Printf("Received new order! ID: %s, Amount: %.2f", event.NewOrder.ID, event.NewOrder.Amount)
	case goldenpay.EventNewMessage:
		log.Printf("New message in chat %s: %s", event.NewMessage.ChatID, event.NewMessage.Text)
	default:
		log.Printf("Received raw event: %s", string(event.Payload.Body))
	}
	return nil
}

func main() {
	config := goldenpay.DefaultWebhookConfig()
	config.BindAddr = "127.0.0.1:8080"
	
	// Create the handler and server
	handler := &MyWebhookHandler{}
	server := goldenpay.NewWebhookServer(config, handler)

	log.Println("Starting Webhook server on :8080...")
	if err := server.Run(); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
