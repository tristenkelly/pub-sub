package pubsub

import (
	"encoding/json"

	"context"

	amqp "github.com/rabbitmq/amqp091-go"
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
