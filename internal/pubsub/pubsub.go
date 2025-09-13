package pubsub

import (
	"encoding/json"

	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type simpleQueueType int

const (
	TransientQueue simpleQueueType = iota
	DurableQueue
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			Body:        data,
			ContentType: "application/json",
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	chann, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	isDurable := (queueType == DurableQueue)
	isTransient := (queueType == TransientQueue)
	queue, err := chann.QueueDeclare(
		queueName,
		isDurable,
		isTransient,
		isTransient,
		false,
		nil,
	)
	if err != nil {
		chann.Close()
		return nil, amqp.Queue{}, err
	}
	err = chann.QueueBind(
		queueName,
		key,
		exchange,
		false,
		nil,
	)
	if err != nil {
		chann.Close()
		return nil, amqp.Queue{}, err
	}
	return chann, queue, nil
}
