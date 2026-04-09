package conexao

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ConexaoRabbitmq abre e retorna apenas a Connection.
// URL passada via parâmetro para garantir controle centralizado (Produção).
func ConexaoRabbitmq(url string) *amqp.Connection {
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("❌ RabbitMQ: falha crítica ao conectar: %s", err)
	}

	log.Println("✅ RabbitMQ: conexão de produção estabelecida")
	return conn
}
