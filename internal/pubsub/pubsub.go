package pubsub

import (
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType string

const (
	Durable   SimpleQueueType = "durable"
	Transient SimpleQueueType = "transient"
)

type AckType string

const (
	Ack         AckType = "ack"
	NackRequeue AckType = "nack_requeue"
	NackDiscard AckType = "nack_discard"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

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

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T) AckType,
) error {
	channel, _, err := DeclareAndBind(
		conn,
		exchange,
		queueName,
		key,
		queueType,
	)
	if err != nil {
		return err
	}

	deliveryCh, err := channel.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go consumeChannel(deliveryCh, handler)
	return nil
}

func consumeChannel[T any](ch <-chan amqp.Delivery, handler func(T) AckType) {
	for message := range ch {
		var data T
		err := json.Unmarshal(message.Body, &data)
		if err != nil {
			fmt.Printf("error unmarshalling data consumed by client: %v\n", err)
		}

		switch ackType := handler(data); ackType {
		case Ack:
			fmt.Println("Sending an Ack...")
			err = message.Ack(false)
		case NackRequeue:
			fmt.Println("Sending a Nack with requeue...")
			err = message.Nack(false, true)
		case NackDiscard:
			fmt.Println("Sending a Nack without requeue...")
			err = message.Nack(false, false)
		}
		if err != nil {
			fmt.Printf("err handling data: %v\n", err)
		}

		err = message.Ack(false)
		if err != nil {
			fmt.Printf("error acknowledging delivery on client: %v\n", err)
		}
	}
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	failOnError(err, "Failed to create a channel")

	// Declare the DLX and DLQ
	err = ch.ExchangeDeclare("peril_dlx", "fanout", true, false, false, false, nil)
	failOnError(err, "Failed to declare the DLX")

	_, err = ch.QueueDeclare("peril_dlq", true, false, false, false, nil)
	failOnError(err, "Failed to declare the DLQ")

	err = ch.QueueBind("peril_dlq", "", "peril_dlx", false, nil)
	failOnError(err, "Failed to bind the DLQ to the DLX")

	args := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}

	queue, err := ch.QueueDeclare(
		queueName,
		queueType == Durable,
		queueType == Transient,
		queueType == Transient,
		false,
		args,
	)
	failOnError(err, "Failed to declare the main queue")

	err = ch.QueueBind(queueName, key, exchange, false, nil)
	return ch, queue, err
}
