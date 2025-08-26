package main

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril client...")

	connection, _, err := pubsub.ConnectToRabbitMQ()
	if err != nil {
		fmt.Printf("Error connecting to RabbitMQ: %v", err)
		return
	}
	defer connection.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("Error creating username: %v", err)
		return
	}

	_, _, err = pubsub.DeclareAndBind(
		connection,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username,
		routing.PauseKey,
		pubsub.Transient,
	)
	if err != nil {
		fmt.Printf("Error declaring/binding to queue: %v", err)
		return
	}

	gamestate := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilDirect,
		"pause."+username,
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gamestate),
	)
	if err != nil {
		fmt.Printf("Error subscribing to channel: %v", err)
		return
	}

	// REPL
	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		command := words[0]

		if command == "spawn" {
			if err := gamestate.CommandSpawn(words); err != nil {
				fmt.Printf("error: %v\n", err)
			}
		} else if command == "move" {
			move, err := gamestate.CommandMove(words)
			if err != nil {
				fmt.Printf("error: %v\n", err)
			}
			fmt.Printf("Move to %s succeeded!\n", move.ToLocation)
		} else if command == "status" {
			gamestate.CommandStatus()
		} else if command == "help" {
			gamelogic.PrintClientHelp()
		} else if command == "spam" {
			fmt.Println("Spamming not allowed yet!")
		} else if command == "quit" {
			gamelogic.PrintQuit()
			break
		} else {
			fmt.Println("Please type a valid command.")
		}
	}

	fmt.Println("Shutting down Peril client...")
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}
