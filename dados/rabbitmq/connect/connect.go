package connect

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ConnectRabbitMQ establishes a connection to RabbitMQ
func ConnectRabbitMQ(url string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ: %s", err)
		return nil, nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Failed to open a channel: %s", err)
		return nil, nil, err
	}

	return conn, ch, nil
}
