package main

import (
	"fmt"
	"strconv"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	"github.com/rabbitmq/amqp091-go"
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

	ch, _ := connection.Channel()
	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		string(routing.ArmyMovesPrefix)+"."+username,
		string(routing.ArmyMovesPrefix)+".*",
		pubsub.Transient,
		handlerMove(gamestate, ch),
	)
	if err != nil {
		fmt.Printf("Error subscribing to move channel: %v", err)
		return
	}

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		string(routing.WarRecognitionsPrefix),
		string(routing.WarRecognitionsPrefix)+".*",
		pubsub.Durable,
		handlerWar(gamestate, ch),
	)
	if err != nil {
		fmt.Printf("Error subscribing to war channel: %v", err)
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
			if len(words) < 2 {
				fmt.Println("spam command expected an arg (count)")
			}

			count, err := strconv.Atoi(words[1])
			if err != nil {
				fmt.Println("spam command requires an integer arg")
				continue
			}

			for range count {
				maliciousLog := gamelogic.GetMaliciousLog()
				pubsub.PublishGameLog(channel, username, maliciousLog)
			}
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

func handlerMove(gs *gamelogic.GameState, ch *amqp091.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)

		switch outcome {
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard

		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack

		case gamelogic.MoveOutcomeMakeWar:
			key := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, gs.Player.Username)
			rec := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.GetPlayerSnap(),
			}
			err := pubsub.PublishJSON(
				ch,
				string(routing.ExchangePerilTopic),
				key,
				rec,
			)
			if err != nil {
				fmt.Printf("Error publishing war message: %v\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack

		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState, ch *amqp091.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(rec gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(rec)

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue

		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard

		case gamelogic.WarOutcomeOpponentWon:
			msg := fmt.Sprintf("%s won a war against %s", winner, loser)
			return pubsub.PublishGameLog(ch, gs.GetUsername(), msg)

		case gamelogic.WarOutcomeYouWon:
			msg := fmt.Sprintf("%s won a war against %s", winner, loser)
			return pubsub.PublishGameLog(ch, gs.GetUsername(), msg)

		case gamelogic.WarOutcomeDraw:
			msg := fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			return pubsub.PublishGameLog(ch, gs.GetUsername(), msg)

		default:
			fmt.Println("error: unrecognized war outcome")
			return pubsub.NackDiscard
		}
	}
}
