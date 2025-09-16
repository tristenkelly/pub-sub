package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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

func handlerWar(ch *amqp.Channel, gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.SimpleAckType {
	return func(decl gamelogic.RecognitionOfWar) pubsub.SimpleAckType {
		defer fmt.Printf("> ")
		outcome, winner, loser := gs.HandleWar(decl)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			entry := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%v won a war against %v", winner, loser),
				Username:    gs.Player.Username,
			}
			err := publishGameLog(ch, entry, gs)
			if err != nil {
				fmt.Printf("error publishing game log: %v\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			entry := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%v won a war against %v", winner, loser),
				Username:    gs.Player.Username,
			}
			err := publishGameLog(ch, entry, gs)
			if err != nil {
				fmt.Printf("error publishing game log: %v\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			entry := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("A war between %v and %v resulted in a draw", winner, loser),
				Username:    gs.Player.Username,
			}
			err := publishGameLog(ch, entry, gs)
			if err != nil {
				fmt.Printf("error publishing game log: %v\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			return pubsub.NackDiscard
		}
	}
}

func publishGameLog(ch *amqp.Channel, entry routing.GameLog, gs *gamelogic.GameState) error {
	err := pubsub.PublishGob(
		ch,
		routing.ExchangePerilTopic,
		routing.GameLogSlug+"."+gs.Player.Username,
		entry,
	)
	if err != nil {
		return err
	}
	return nil
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
		handlerWar(ch, gameState),
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
			if len(words) < 2 {
				fmt.Printf("usage: spam <number>\n")
				continue
			}
			n, err := strconv.Atoi(words[1])
			if err != nil || n <= 0 {
				fmt.Printf("invalid number: %s\n", words[1])
				continue
			}

			for i := 0; i < n; i++ {
				entry := gamelogic.GetMaliciousLog()
				gl := routing.GameLog{
					CurrentTime: time.Now(),
					Message:     entry,
					Username:    gameState.Player.Username,
				}
				err := pubsub.PublishGob(
					ch,
					routing.ExchangePerilTopic,
					routing.GameLogSlug+"."+gameState.Player.Username,
					gl,
				)
				if err != nil {
					fmt.Printf("error publishing game log: %v\n", err)
					continue
				}
			}
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		default:
			fmt.Printf("unknown command: %s\n", cmd)
		}
	}
}
