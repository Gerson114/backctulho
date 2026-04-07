package handlers

import (
	"context"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ProcessVoteBatch é o manipulador responsável por pegar os lotes do RabbitMQ
// e dispará-los para o banco de dados (ex: Postgres Aurora)
func ProcessVoteBatch(ctx context.Context, batch []amqp.Delivery) error {
	log.Printf("[Processamento] Recebido lote de %d votos.", len(batch))

	for i, msg := range batch {
		// Logando o conteúdo de cada mensagem (voto) no terminal
		log.Printf("[Voto %d] Conteúdo: %s", i+1, string(msg.Body))
	}

	log.Printf("[Processamento] Finalizado o log de %d votos. Enviando ACK para o RabbitMQ...", len(batch))

	// Quando esta função retorna nil (sem erro), o Worker envia o Ack para o RabbitMQ
	return nil
}
