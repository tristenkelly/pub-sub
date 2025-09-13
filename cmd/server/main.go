package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/streadway/amqp"
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

	go func() { //since we're blocking until signal, we don't need to defer conn.Close()
		<-sigChan
		fmt.Println("Shutting down server...")
		conn.Close()
		os.Exit(0)
	}()
	select {} //ensure we're blocking until all signals received
}
