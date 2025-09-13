package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
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

	go func() { //since we're blocking until signal, we don't need to defer conn.Close()
		<-sigChan
		fmt.Println("Shutting down server...")
		conn.Close()
		os.Exit(0)
	}()
	select {}
}
