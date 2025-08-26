package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	"github.com/rabbitmq/amqp091-go"
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
	gamelogic.PrintServerHelp()

	_, _, err = pubsub.DeclareAndBind(
		connection,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+".*",
		pubsub.Durable,
	)
	if err != nil {
		fmt.Printf("Error declaring/binding to queue: %v", err)
		return
	}

	// REPL
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}
		word := input[0]
		if word == "pause" {
			fmt.Println("Pausing the game...")
			if err := ToggleGameState(channel, true); err != nil {
				fmt.Printf("error: %v", err)
				os.Exit(1)
			}

		} else if word == "resume" {
			fmt.Println("Resuming the game...")
			if err := ToggleGameState(channel, false); err != nil {
				fmt.Printf("error: %v", err)
				os.Exit(1)
			}
		} else if word == "quit" {
			fmt.Println("Exiting the game...")
			break
		} else {
			fmt.Println("Please type a valid command.")
		}
	}

	fmt.Println("Shutting down Peril server...")
}

func ToggleGameState(channel *amqp091.Channel, pause bool) error {
	data, err := json.Marshal(routing.PlayingState{IsPaused: pause})
	if err != nil {
		return err
	}
	return pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, data)
}
