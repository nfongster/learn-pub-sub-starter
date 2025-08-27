package main

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
)

func main() {
	fmt.Println("Starting Peril client...")

	connection, channel, err := pubsub.ConnectToRabbitMQ()
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

	// Bind to channels
	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilDirect,
		"pause."+username,
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gamestate),
	)
	if err != nil {
		fmt.Printf("Error subscribing to pause channel: %v", err)
		return
	}

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		string(routing.ArmyMovesPrefix)+"."+username,
		string(routing.ArmyMovesPrefix)+".*",
		pubsub.Transient,
		handlerMove(gamestate),
	)
	if err != nil {
		fmt.Printf("Error subscribing to move channel: %v", err)
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
			} else {
				fmt.Printf("Move to %s succeeded!\n", move.ToLocation)
			}

			// publish move
			err = pubsub.PublishJSON(
				channel,
				string(routing.ExchangePerilTopic),
				string(routing.ArmyMovesPrefix)+"."+username,
				move)
			if err != nil {
				fmt.Printf("error: %v\n", err)
			} else {
				fmt.Println("successfully published move!")
			}
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			return pubsub.Ack
		default:
			return pubsub.NackDiscard
		}
	}
}
