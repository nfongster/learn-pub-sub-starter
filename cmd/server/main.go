package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")

	connectionStr := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(connectionStr)

	if err != nil {
		fmt.Printf("Error connecting to server: %v", err)
		return
	}
	defer connection.Close()

	fmt.Println("Connection was successful!")

	channel, err := connection.Channel()
	if err != nil {
		fmt.Printf("Error creating a channel: %v", err)
		return
	}

	data, err := json.Marshal(routing.PlayingState{IsPaused: true})
	if err != nil {
		fmt.Printf("Error marshalling data: %v", err)
		return
	}
	if err = pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, data); err != nil {
		fmt.Printf("Error publishing data to exchange: %v", err)
		return
	}

	// wait for ctrl-c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	fmt.Println("Shutting down Peril server...")
}
