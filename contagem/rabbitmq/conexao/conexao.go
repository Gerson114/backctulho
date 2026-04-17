package conexao

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func Conectar(url string) *amqp.Connection {
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("❌ RabbitMQ: falha ao conectar: %v", err)
	}
	log.Println("✅ RabbitMQ: conexão estabelecida")
	return conn
}
