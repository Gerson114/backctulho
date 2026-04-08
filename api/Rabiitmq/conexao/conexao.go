package conexao

import (
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ConexaoRabbitmq abre e retorna apenas a Connection.
// Os canais são criados pelo WorkerPool — um por goroutine.
// URL lida da variável de ambiente RABBITMQ_URL.
func ConexaoRabbitmq() *amqp.Connection {
	url := os.Getenv("RABBITMQ_URL")

	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("RabbitMQ: erro ao conectar: %s", err)
	}

	log.Println("RabbitMQ: conectado com sucesso")
	return conn
}
