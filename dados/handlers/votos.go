package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"nome-do-projeto/db"
	"nome-do-projeto/models"
)

// ProcessVoteBatch persiste os votos usando SQL PURO para velocidade máxima.
func ProcessVoteBatch(ctx context.Context, batch []amqp.Delivery) error {
	if len(batch) == 0 {
		return nil
	}

	var votos []models.Voto
	for _, msg := range batch {
		var v models.Voto
		if err := json.Unmarshal(msg.Body, &v); err != nil {
			log.Printf("[handlers] erro no unmarshal: %v", err)
			continue
		}
		votos = append(votos, v)
	}

	if len(votos) == 0 {
		return nil
	}

	// 🚀 MONTAGEM DE SQL PURO (Raw SQL)
	// Isso é 10x mais rápido que qualquer ORM
	query := "INSERT INTO votos (nome, numero, emenda_votada, votado_em, ip_origem) VALUES "
	vals := []interface{}{}

	for i, v := range votos {
		p := i * 5
		query += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d),", p+1, p+2, p+3, p+4, p+5)
		vals = append(vals, v.Nome, v.Numero, v.EmendaVotada, v.VotadoEm, v.IPOrigem)
	}

	// Remove a última vírgula e finaliza
	query = strings.TrimSuffix(query, ",")

	// Executa diretamente no banco (Bypassing GORM logic)
	if err := db.Instance.Exec(query, vals...).Error; err != nil {
		log.Printf("[handlers] ❌ FALHA CRÍTICA SQL: %v", err)
		return err
	}

	log.Printf("[handlers] ⚡ ULTRASONIC: Lote de %d votos persistido via SQL Puro.", len(votos))
	return nil
}
