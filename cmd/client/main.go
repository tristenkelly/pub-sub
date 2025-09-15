package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.SimpleAckType {
	return func(state routing.PlayingState) pubsub.SimpleAckType {
		defer fmt.Printf("> ")
		gs.HandlePause(state)
		return pubsub.Ack
	}
}

func handlerMove(ch *amqp.Channel, gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.SimpleAckType {
	return func(move gamelogic.ArmyMove) pubsub.SimpleAckType {
		defer fmt.Printf("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+gs.Player.Username,
				gamelogic.RecognitionOfWar{
					Attacker: gs.Player,
					Defender: move.Player,
				},
			)
			if err != nil {
				fmt.Printf("error publishing war declaration: %v\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.SimpleAckType {
	return func(decl gamelogic.RecognitionOfWar) pubsub.SimpleAckType {
		defer fmt.Printf("> ")
		outcome, _, _ := gs.HandleWar(decl)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeYouWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			return pubsub.NackDiscard
		}
	}
}

func main() {
	fmt.Println("Starting Peril client...")
	conStr := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(conStr)
	if err != nil {
		log.Printf("error connecting to server: %v", err)
		return
	}
	fmt.Println("Connected to server")

	user, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Printf("error during client welcome: %v", err)
		return
	}

	queueName := fmt.Sprintf("%v.%s", routing.PauseKey, user)

	ch, queue, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		queueName,
		routing.PauseKey,
		pubsub.TransientQueue,
	)
	if err != nil {
		log.Printf("error declaring and binding: %v", err)
		return
	}
	fmt.Printf("Declared and bound queue %s\n", queue.Name)

	defer ch.Close()

	gameState := gamelogic.NewGameState(user)

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		queueName,
		routing.PauseKey,
		pubsub.TransientQueue,
		handlerPause(gameState),
	)
	if err != nil {
		log.Printf("error subscribing to JSON: %v", err)
		return
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+user,
		routing.ArmyMovesPrefix+".*",
		pubsub.TransientQueue,
		handlerMove(ch, gameState),
	)
	if err != nil {
		log.Printf("error subscribing to JSON: %v", err)
		return
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,
		routing.WarRecognitionsPrefix+".*",
		pubsub.DurableQueue,
		handlerWar(gameState),
	)
	if err != nil {
		log.Printf("error subscribing to JSON: %v", err)
		return
	}

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		cmd := words[0]
		switch cmd {
		case "move":
			move, err := gameState.CommandMove(words)
			if err != nil {
				fmt.Printf("error executing move command: %v\n", err)
			}
			err = pubsub.PublishJSON(ch,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+user,
				move,
			)
			if err != nil {
				fmt.Printf("error publishing move command: %v\n", err)
			}
		case "spawn":
			err := gameState.CommandSpawn(words)
			if err != nil {
				fmt.Printf("error executing spawn command: %v\n", err)
			}
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Printf("spamming not allowed yet!\n")
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		default:
			fmt.Printf("unknown command: %s\n", cmd)
		}
	}
}
