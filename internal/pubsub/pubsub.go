package pubsub

import (
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type simpleQueueType string

const (
	Durable   simpleQueueType = "durable"
	Transient simpleQueueType = "transient"
)

func ConnectToRabbitMQ() (*amqp.Connection, *amqp.Channel, error) {
	connectionStr := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(connectionStr)

	if err != nil {
		return connection, nil, fmt.Errorf("error connecting to server: %v", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		return connection, channel, fmt.Errorf("error creating a channel: %v", err)
	}
	return connection, channel, nil
}

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		return err
	}

	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        bytes,
	}
	return ch.Publish(exchange, key, false, false, msg)
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {
	channel, err := conn.Channel()
	if err != nil {
		return channel, amqp.Queue{}, err
	}

	queue, err := channel.QueueDeclare(
		queueName,
		queueType == Durable,
		queueType == Transient,
		queueType == Transient,
		false,
		nil,
	)
	if err != nil {
		return channel, queue, err
	}

	err = channel.QueueBind(queueName, key, exchange, false, nil)
	return channel, queue, err
}
