// Example demonstrates basic goldenpay-go usage.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rxzsu/goldenpay-go"
)

func main() {
	key := os.Getenv("GOLDEN_KEY")
	if key == "" {
		log.Fatal("GOLDEN_KEY environment variable required")
	}

	cfg := goldenpay.NewConfig(key)
	client, err := goldenpay.New(cfg)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	session, err := client.Connect()
	if err != nil {
		log.Fatalf("connect: %v", err)
	}

	fmt.Printf("Authenticated as %s (ID %d)\n", session.User().Username, session.User().ID)

	// Fetch orders
	orders, err := session.FetchOrders()
	if err != nil {
		log.Printf("fetch orders: %v", err)
	} else {
		fmt.Printf("Active orders: %d\n", len(orders))
		for _, o := range orders {
			fmt.Printf("  %s - %s (%s)\n", o.ID, o.Description, o.Status)
		}
	}

	// Fetch balance
	balance, err := session.FetchBalance()
	if err != nil {
		log.Printf("fetch balance: %v", err)
	} else {
		fmt.Printf("Balance: %.2f\n", balance)
	}

	// Bot example (poll for new orders/messages)
	bot := goldenpay.NewBot(session).WithOptions(goldenpay.DefaultBotOptions())

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Bootstrap to avoid re-processing existing orders
	if err := bot.Bootstrap(); err != nil {
		log.Printf("bootstrap: %v", err)
	}

	fmt.Println("Bot running. Press Ctrl+C to stop.")
	if err := bot.Run(ctx, func(ev goldenpay.GoldenPayEvent) error {
		if ev.NewOrder != nil {
			fmt.Printf("New order: %s - %s\n", ev.NewOrder.ID, ev.NewOrder.Description)
		}
		if ev.NewMessage != nil {
			fmt.Printf("New message from user %d: %s\n", ev.NewMessage.AuthorID, ev.NewMessage.Text)
		}
		return nil
	}); err != nil {
		log.Fatalf("bot error: %v", err)
	}
}
