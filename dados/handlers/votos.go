package handlers

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"nome-do-projeto/models"
)

// ProcessVoteBatch logs the received votes instead of persisting them.
func ProcessVoteBatch(ctx context.Context, batch []amqp.Delivery) error {
	var votos []models.Voto
	for _, msg := range batch {
		var v models.Voto
		if err := json.Unmarshal(msg.Body, &v); err != nil {
			log.Printf("[handlers] erro ao fazer Unmarshal da mensagem: %v", err)
			continue
		}
		votos = append(votos, v)
	}

	if len(votos) == 0 {
		log.Println("[handlers] lote vazio, nada a registrar.")
		return nil
	}

	log.Printf("[handlers] lote de %d votos recebido. Primeiro voto: %+v", len(votos), votos[0])
	return nil
}
