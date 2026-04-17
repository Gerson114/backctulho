package handlers

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"nome-do-projeto/db"
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

	// 🚀 PERSISTÊNCIA EM LOTE (Alta Performance)
	// CreateInBatches divide os 2000 votos em pedaços de 500 para o Postgres
	if err := db.Instance.CreateInBatches(votos, 500).Error; err != nil {
		log.Printf("[handlers] ❌ Erro ao salvar lote no banco: %v", err)
		return err
	}

	log.Printf("[handlers] ✅ Sucesso: Lote de %d votos persistido no Postgres.", len(votos))
	return nil
}
