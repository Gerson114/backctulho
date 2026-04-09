package consumer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"nome-do-projeto/rabbitmq/connect"
	"nome-do-projeto/rabbitmq/workers"

	amqp "github.com/rabbitmq/amqp091-go"
)

// StartConsumer abre N canais AMQP independentes e inicia um collector por canal.
// Isso é o equivalente a rodar N consumers em paralelo na mesma conexão TCP.
func StartConsumer(
	ctx context.Context,
	conn *amqp.Connection,
	queueName string,
	prefetchCount int,
	numCollectors int, // canais AMQP paralelos
	numProcessors int, // goroutines de handler
	batchSize int,
	timeout time.Duration,
	handler workers.BatchHandler,
) (*sync.WaitGroup, error) {

	channels := make([]<-chan amqp.Delivery, 0, numCollectors)

	for i := 1; i <= numCollectors; i++ {
		// Cada collector tem seu próprio canal AMQP
		ch, err := connect.OpenChannel(conn)
		if err != nil {
			return nil, fmt.Errorf("canal %d: %w", i, err)
		}

		// Declara a fila em cada canal (idempotente)
		q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		if err != nil {
			return nil, fmt.Errorf("canal %d: declare queue: %w", i, err)
		}

		// QoS por canal — cada canal pré-busca até prefetchCount msgs
		err = ch.Qos(prefetchCount, 0, false)
		if err != nil {
			return nil, fmt.Errorf("canal %d: set QoS: %w", i, err)
		}

		// Registra o consumer neste canal
		msgs, err := ch.Consume(q.Name, fmt.Sprintf("collector-%d", i), false, false, false, false, nil)
		if err != nil {
			return nil, fmt.Errorf("canal %d: consume: %w", i, err)
		}

		channels = append(channels, msgs)
		log.Printf("[AMQP] Collector %d ativo | Fila: %s | Prefetch: %d", i, queueName, prefetchCount)
	}

	log.Printf("[SISTEMA] 🚀 Pipeline: %d collectors × %d processors | Lote %d | Timeout %s",
		numCollectors, numProcessors, batchSize, timeout)

	wg := workers.StartWorkerPool(ctx, numCollectors, numProcessors, batchSize, timeout, channels, handler)
	return wg, nil
}
