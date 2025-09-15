package pubsub

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"

	"context"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

type simpleQueueType int

const (
	TransientQueue simpleQueueType = iota
	DurableQueue
)

type SimpleAckType int

const (
	Ack SimpleAckType = iota
	NackRequeue
	NackDiscard
)

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) SimpleAckType,
) error {
	chann, q, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}
	msgs, err := chann.Consume(
		q.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		chann.Close()
		return err
	}
	go func() {
		for d := range msgs {
			var val T
			if err := json.Unmarshal(d.Body, &val); err == nil {
				switch handler(val) {
				case Ack:
					d.Ack(false)
					fmt.Println("Acked message")
				case NackRequeue:
					d.Nack(false, true)
					fmt.Println("Nacked message, requeued")
				case NackDiscard:
					d.Nack(false, false)
					fmt.Println("Nacked message, discarded")
				}
			}
		}
	}()
	return nil
}

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
		amqp.Table{"x-dead-letter-exchange": routing.ExchangePerilDeadLetter},
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

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(val); err != nil {
		return err
	}
	err := ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			Body:        buf.Bytes(),
			ContentType: "application/gob",
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType,
	handler func(T) SimpleAckType,
) error {
	chann, q, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}
	msgs, err := chann.Consume(
		q.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		chann.Close()
		return err
	}
	go func() {
		for d := range msgs {
			var val T
			dec := gob.NewDecoder(bytes.NewReader(d.Body))
			if err := dec.Decode(&val); err == nil {
				switch handler(val) {
				case Ack:
					d.Ack(false)
					fmt.Println("Acked message")
				case NackRequeue:
					d.Nack(false, true)
					fmt.Println("Nacked message, requeued")
				case NackDiscard:
					d.Nack(false, false)
					fmt.Println("Nacked message, discarded")
				}
			}
		}
	}()
	return nil
}
