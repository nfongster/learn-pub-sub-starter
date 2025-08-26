package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril server...")

	connection, channel, err := pubsub.ConnectToRabbitMQ()
	if err != nil {
		fmt.Printf("Error connecting to RabbitMQ: %v", err)
		return
	}
	fmt.Println("Connection was successful!")
	defer connection.Close()

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
