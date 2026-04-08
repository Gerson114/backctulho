package consumer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"nome-do-projeto/rabbitmq/workers" // ajuste o path conforme seu projeto

	amqp "github.com/rabbitmq/amqp091-go"
)

func StartConsumer(ctx context.Context, ch *amqp.Channel, queueName string, prefetchCount int, numWorkers int, batchSize int, timeout time.Duration, handler workers.BatchHandler) (*sync.WaitGroup, error) {
	// 1. Declaração da Fila
	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	// 2. QoS AGRESSIVO (O segredo dos 10k/s)
	// Permitimos 10.000 mensagens unacked no canal para eliminar latência de rede.
	// prefetchCount is supplied via configuration
	err = ch.Qos(prefetchCount, 0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	// 3. Registro do Consumidor
	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to register a consumer: %w", err)
	}

	log.Printf("[SISTEMA] Modo Turbo: Prefetch %d | Workers %d | Lote %d", prefetchCount, numWorkers, batchSize)

	// 4. Inicia o Pool de Workers
	wg := workers.StartWorkerPool(ctx, numWorkers, batchSize, timeout, msgs, handler)

	return wg, nil
}
