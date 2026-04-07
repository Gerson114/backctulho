package workerpool

import (
	"context"
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// worker é executado em uma goroutine independente.
// Cada worker abre seu próprio canal RabbitMQ — sem concorrência, sem race condition.
func (wp *WorkerPool) worker(id int) {
	for {
		canal, err := wp.conn.Channel()
		if err != nil {
			log.Printf("worker %d: erro ao abrir canal, tentando novamente em 3s: %s", id, err)
			time.Sleep(3 * time.Second)
			continue
		}

		_, err = canal.QueueDeclare(nomeFila, true, false, false, false, nil)
		if err != nil {
			log.Printf("worker %d: erro ao declarar fila: %s", id, err)
			canal.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		log.Printf("worker %d: pronto para receber votos", id)
		wp.processar(id, canal)

		// Só chega aqui se o canal fechou — tenta reconectar
		canal.Close()
		log.Printf("worker %d: canal fechado, reconectando...", id)
		time.Sleep(1 * time.Second)
	}
}

// processar fica em loop consumindo a fila interna e publicando no RabbitMQ.
// Retorna quando o canal é fechado ou ocorre erro de publicação.
func (wp *WorkerPool) processar(id int, canal *amqp.Channel) {
	for voto := range wp.fila {
		corpo, err := json.Marshal(voto)
		if err != nil {
			log.Printf("worker %d: erro ao serializar voto (descartado): %s", id, err)
			continue
		}

		err = canal.PublishWithContext(
			context.Background(),
			"",
			nomeFila,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Body:         corpo,
			},
		)
		if err != nil {
			log.Printf("worker %d: erro ao publicar, recolocando voto na fila: %s", id, err)
			// Tenta recolocar o voto na fila para não perder
			select {
			case wp.fila <- voto:
			default:
				log.Printf("worker %d: fila cheia, voto perdido", id)
			}
			return // sai do loop para reconectar o canal
		}
	}
}
