package workers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type BatchHandler func(ctx context.Context, batch []amqp.Delivery) error

// StartWorkerPool inicia o pipeline de alta performance com agregação global (Funil).
//
// Arquitetura de Funil:
//
//	[RabbitMQ] ──→ [Collector-1] ─┐
//	[RabbitMQ] ──→ [Collector-2] ──┼──→ [rawMsgsCh (Central)] ──→ [GLOBAL AGGREGATOR] ──→ [batchCh] ──→ [Processors]
//	[RabbitMQ] ──→ [Collector-N] ─┘
//
// O Agregador Global garante que os lotes sejam sempre preenchidos ao máximo antes de ir para o banco.
func StartWorkerPool(
	ctx context.Context,
	numCollectors int,
	numProcessors int,
	batchSize int,
	timeout time.Duration,
	channels []<-chan amqp.Delivery,
	handler BatchHandler,
) *sync.WaitGroup {
	var wg sync.WaitGroup

	if timeout <= 0 {
		timeout = 1 * time.Second
	}

	// 1. Canal de Mensagens Brutas (O Funil Central)
	rawMsgsCh := make(chan amqp.Delivery, batchSize*5)

	// 2. Canal de Lote Pronto para os Processadores
	batchCh := make(chan []amqp.Delivery, numProcessors*4)

	// Contador atômico
	var totalProcessed int64

	// ── FASE 1: COLETORES ────────────────────────────────────────────────────────
	// Apenas puxam do RabbitMQ e jogam no Funil Central
	var collectorWg sync.WaitGroup
	for i, msgs := range channels {
		collectorWg.Add(1)
		go func(id int, messages <-chan amqp.Delivery) {
			defer collectorWg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-messages:
					if !ok {
						return
					}
					select {
					case rawMsgsCh <- msg:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i+1, msgs)
	}

	// ── FASE 2: AGREGADOR GLOBAL ──────────────────────────────────────────────
	// O cérebro do sistema: junta tudo em lotes gigantes e eficientes
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(batchCh)

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

		// Fecha quando todos os coletores pararem
		collectorsDone := make(chan struct{})
		go func() {
			collectorWg.Wait()
			log.Println("[SISTEMA] 🛑 Coletores pararam. Fechando canal de mensagens brutas...")
			close(rawMsgsCh)
			close(collectorsDone)
		}()

		idleTicker := time.NewTicker(10 * time.Second)
		defer idleTicker.Stop()

		for {
			select {
			case msg, ok := <-rawMsgsCh:
				if !ok {
					flush()
					log.Println("[SISTEMA] 🏁 Agregador Global finalizado.")
					return
				}
				batch = append(batch, msg)
				if len(batch) >= batchSize {
					flush()
				}
			case <-ticker.C:
				flush()
			case <-idleTicker.C:
				if len(batch) == 0 {
					log.Println("[SISTEMA] 💤 Aguardando novos votos do RabbitMQ...")
				}
			case <-ctx.Done():
				flush()
				return
			}
		}
	}()

	// ── FASE 3: PROCESSADORES ──────────────────────────────────────────────────
	// Multitasking de gravação no banco
	for i := 1; i <= numProcessors; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for batch_data := range batchCh {
				// 1. Processa a carga no banco
				if err := handler(ctx, batch_data); err != nil {
					fmt.Printf("[Processor %d] ⚠️  Erro: %v\n", id, err)
					continue // Não dá ACK se falhar brutalmente
				}

				// 2. ACK múltiplo no último msg do lote (Só após sucesso no banco)
				if len(batch_data) > 0 {
					_ = batch_data[len(batch_data)-1].Ack(true)
				}

				n := atomic.AddInt64(&totalProcessed, int64(len(batch_data)))
				fmt.Printf("[%s] ⚡ Processor %02d | Lote: %4d msgs | Total: %d\n",
					time.Now().Format("15:04:05"), id, len(batch_data), n)
			}
		}(i)
	}

	return &wg
}
