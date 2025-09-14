package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
	gamelogic.PrintServerHelp()
	conStr := "amqp://guest:guest@localhost:5672/"
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	conn, err := amqp.Dial(conStr)
	if err != nil {
		log.Fatal("Failed to connect to rabbitMQ", err)
		return
	}
	fmt.Println("Connected to server")

	chann, err := conn.Channel()
	if err != nil {
		log.Fatal("Failed to open a channel", err)
		return
	}
	fmt.Println("Channel opened")

	err = pubsub.PublishJSON(chann, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
	if err != nil {
		log.Fatal("Failed to publish message", err)
		return
	}

	ch, q, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+".*",
		pubsub.DurableQueue,
	)

	if err != nil {
		log.Fatal("Failed to declare and bind queue", err)
		return
	}

	fmt.Printf("Declared and bound queue %s\n", q.Name)

	defer ch.Close()

	go func() { //since we're blocking until signal, we don't need to defer conn.Close()
		<-sigChan
		fmt.Println("Shutting down server...")
		conn.Close()
		os.Exit(0)
	}()

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}
		cmd := words[0]
		switch cmd {
		case "pause":
			err = pubsub.PublishJSON(chann, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			if err != nil {
				log.Println("Failed to publish message", err)
			} else {
				fmt.Println("Game paused")
			}
		case "resume":
			err = pubsub.PublishJSON(chann, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			if err != nil {
				log.Println("Failed to publish message", err)
			} else {
				fmt.Println("Game resumed")
			}
		case "quit":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Unknown command:", cmd)
		}
	}
}
