package producer

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"meu-projeto/models"
)

const nomeFila = "votos"

// Producer encapsula o canal RabbitMQ para publicação de mensagens.
type Producer struct {
	canal *amqp.Channel
}

// Novo cria um Producer. Chame uma vez no main.go, passando o canal já aberto.
func Novo(canal *amqp.Channel) *Producer {
	return &Producer{canal: canal}
}

// Publicar serializa o voto em JSON e o envia para a fila de forma persistente.
// Retorna erro se a serialização ou a publicação falharem.
func (p *Producer) Publicar(ctx context.Context, voto models.Voto) error {
	corpo, err := json.Marshal(voto)
	if err != nil {
		return fmt.Errorf("producer: falha ao serializar voto: %w", err)
	}

	err = p.canal.PublishWithContext(
		ctx,
		"",       // exchange padrão
		nomeFila, // routing key = nome da fila
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // sobrevive ao restart do broker
			Body:         corpo,
		},
	)
	if err != nil {
		return fmt.Errorf("producer: falha ao publicar na fila %q: %w", nomeFila, err)
	}

	return nil
}
