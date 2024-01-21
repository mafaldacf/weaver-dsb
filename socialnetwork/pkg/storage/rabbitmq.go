package storage

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func RabbitMQClient (ctx context.Context, address string, port int) (*amqp.Channel, *amqp.Connection, error) {
	uri := fmt.Sprintf("amqp://%s:%s@%s:%d/", "admin", "admin", address, port)
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, nil, fmt.Errorf("error establishing connection with rabbitmq: %s", err.Error())
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("error openning channel for rabbitmq: %s", err.Error())
	}
	return ch, conn, nil
}
