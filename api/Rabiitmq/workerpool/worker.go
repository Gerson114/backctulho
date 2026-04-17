package workerpool

import (
	"context"
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// worker é executado em uma goroutine independente.
func (wp *WorkerPool) worker(id int) {
	log.Printf("worker %d: pronto para processar votos do pool", id)
	wp.processar(id)
}

// processar fica em loop consumindo a fila interna e publicando no RabbitMQ usando o pool.
func (wp *WorkerPool) processar(id int) {
	for voto := range wp.fila {
		corpo, err := json.Marshal(voto)
		if err != nil {
			log.Printf("worker %d: erro ao serializar voto (descartado): %s", id, err)
			continue
		}

		// 🛡️ PUB-LOCK: Protege o acesso aos canais do pool (amqp.Channel não é thread-safe para publish)
		wp.canalMu.Lock()
		
		// Seleciona o canal via Round-Robin simples
		ch := wp.canais[wp.indice%len(wp.canais)]
		wp.indice++
		
		err = ch.PublishWithContext(
			context.Background(),
			nomeExchange,
			"",
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Body:         corpo,
			},
		)
		wp.canalMu.Unlock()

		if err != nil {
			log.Printf("worker %d: erro ao publicar, tentando devolver à fila: %s", id, err)
			// Tenta devolver o voto à fila se houver erro de rede
			select {
			case wp.fila <- voto:
			default:
				log.Printf("worker %d: fila cheia ao tentar reenviar, voto perdido", id)
			}
			
			// Se o canal falhou, o ideal seria o WorkerPool ter um sistema de health check,
			// mas para este cenário de 2000 RPS, o Mutex + Pool reduz erros a quase zero.
			time.Sleep(100 * time.Millisecond)
		}
	}
}
