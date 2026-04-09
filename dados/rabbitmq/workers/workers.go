package workers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type BatchHandler func(ctx context.Context, batch []amqp.Delivery) error

// StartWorkerPool inicia o pipeline de alta performance com múltiplos collectors.
//
// Arquitetura:
//   [RabbitMQ] ──→ [Collector-1 (canal próprio)] ─┐
//   [RabbitMQ] ──→ [Collector-2 (canal próprio)] ──┼──→ [batchCh buffered] ──→ [50x Processors]
//   [RabbitMQ] ──→ [Collector-N (canal próprio)] ─┘
//
// Cada collector tem seu próprio canal AMQP → sem serialização de rede.
// Os processors processam lotes em paralelo chamando o handler real.
func StartWorkerPool(
	ctx context.Context,
	numCollectors int,   // canais AMQP paralelos (era numWorkers - agora separado)
	numProcessors int,   // goroutines de handler em paralelo
	batchSize int,
	timeout time.Duration,
	channels []<-chan amqp.Delivery, // um chan por collector
	handler BatchHandler,
) *sync.WaitGroup {
	var wg sync.WaitGroup

	if timeout <= 0 {
		timeout = 100 * time.Millisecond
	}

	// Canal de lotes — buffer generoso para não bloquear nenhum collector
	batchCh := make(chan []amqp.Delivery, numProcessors*8)

	// Contador atômico de mensagens processadas
	var totalProcessed int64

	// ── 1. COLLECTORS ────────────────────────────────────────────────────────────
	// Uma goroutine por canal AMQP. Cada uma lê independentemente, sem lock.
	var collectorWg sync.WaitGroup
	for i, msgs := range channels {
		collectorWg.Add(1)
		go func(id int, messages <-chan amqp.Delivery) {
			defer collectorWg.Done()

			batch := make([]amqp.Delivery, 0, batchSize)
			ticker := time.NewTicker(timeout)
			defer ticker.Stop()

			flush := func() {
				if len(batch) == 0 {
					return
				}
				toSend := make([]amqp.Delivery, len(batch))
				copy(toSend, batch)
				select {
				case batchCh <- toSend:
				case <-ctx.Done():
				}
				batch = batch[:0]
				ticker.Reset(timeout)
			}

			for {
				select {
				case <-ctx.Done():
					flush()
					return

				case msg, ok := <-messages:
					if !ok {
						flush()
						return
					}
					batch = append(batch, msg)
					if len(batch) >= batchSize {
						flush()
					}

				case <-ticker.C:
					flush()
				}
			}
		}(i+1, msgs)
	}

	// Fecha batchCh quando todos os collectors terminarem
	wg.Add(1)
	go func() {
		defer wg.Done()
		collectorWg.Wait()
		close(batchCh)
	}()

	// ── 2. PROCESSORS ────────────────────────────────────────────────────────────
	// N goroutines independentes processam lotes em paralelo.
	for i := 1; i <= numProcessors; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for batch := range batchCh {
				// ACK múltiplo no último msg do lote (uma única roundtrip de rede!)
				if len(batch) > 0 {
					_ = batch[len(batch)-1].Ack(true) // multiple=true → ack todos anteriores
				}

				if err := handler(ctx, batch); err != nil {
					fmt.Printf("[Processor %d] ⚠️  Erro no handler: %v\n", id, err)
				}

				n := atomic.AddInt64(&totalProcessed, int64(len(batch)))
				fmt.Printf("[%s] ⚡ Processor %02d | Lote: %4d msgs | Total: %d\n",
					time.Now().Format("15:04:05"), id, len(batch), n)
			}
		}(i)
	}

	return &wg
}
