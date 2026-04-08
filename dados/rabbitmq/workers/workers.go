package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type BatchHandler func(ctx context.Context, batch []amqp.Delivery) error

func StartWorkerPool(ctx context.Context, numWorkers int, batchSize int, timeout time.Duration, messages <-chan amqp.Delivery, handler BatchHandler) *sync.WaitGroup {
	var wg sync.WaitGroup
	esteira := make(chan int, numWorkers*5) // Passamos apenas o tamanho do lote para o log

	// 1. Logger de Alta Performance (Consumidor da Esteira)
	// Centraliza os logs para não travar os workers de coleta
	wg.Add(1)
	go func() {
		defer wg.Done()
		for size := range esteira {
			fmt.Printf("[%s] 🚀 LOTE PROCESSADO: %d mensagens consumidas.\n", time.Now().Format("15:04:05"), size)
		}
	}()

	// 2. Workers de Coleta
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			batchCount := 0
			// Ensure a positive interval for the ticker; fallback to 1s if mis‑configured
			effectiveTimeout := timeout
			if effectiveTimeout <= 0 {
				effectiveTimeout = time.Second
			}
			ticker := time.NewTicker(effectiveTimeout)
			defer ticker.Stop()

			flush := func() {
				if batchCount == 0 {
					return
				}
				esteira <- batchCount // Joga o aviso na esteira
				batchCount = 0
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

					// Confirmamos a mensagem imediatamente para esvaziar a fila
					_ = msg.Ack(false)
					batchCount++

					if batchCount >= batchSize {
						flush()
					}
				case <-ticker.C:
					flush()
				}
			}
		}(i)
	}

	return &wg
}
