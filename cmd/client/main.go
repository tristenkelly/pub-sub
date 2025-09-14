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

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		cmd := words[0]
		switch cmd {
		case "move":
			_, err := gameState.CommandMove(words)
			if err != nil {
				fmt.Printf("error executing move command: %v\n", err)
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
