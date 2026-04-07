package consumer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"nome-do-projeto/rabbitmq/workers"

	amqp "github.com/rabbitmq/amqp091-go"
)

// StartConsumer configures the queue, sets QoS, and starts the batching worker pool.
func StartConsumer(ctx context.Context, ch *amqp.Channel, queueName string, numWorkers int, batchSize int, timeout time.Duration, handler workers.BatchHandler) (*sync.WaitGroup, error) {
	// 1. Declare the queue
	q, err := ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	// 2. Set QoS (Prefetch Count).
	prefetch := (numWorkers * batchSize) * 2
	if prefetch > 60000 {
		prefetch = 60000 // Prevenção limite do protocolo AMQP (16-bit uint per channel max 65535)
	}

	err = ch.Qos(
		prefetch, // prefetch count
		0,        // prefetch size
		false,    // global
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// 3. Start Consuming
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer tag
		false,  // auto-ack (AGORA É FALSO!) O worker quem confima o Lote quando salva com sucesso. Segurança total!
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register a consumer: %w", err)
	}

	// 4. Dispatch messages to the Batching worker pool
	log.Printf("Iniciando pool com %d workers, lendo de %d em %d...", numWorkers, batchSize, batchSize)
	wg := workers.StartWorkerPool(ctx, numWorkers, batchSize, timeout, msgs, handler)

	return wg, nil
}
