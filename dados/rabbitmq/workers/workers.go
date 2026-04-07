package workers

import (
	"context"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// BatchHandler define a função que salvará um loto inteiro de mensagens de uma vez.
type BatchHandler func(ctx context.Context, batch []amqp.Delivery) error

// StartWorkerPool inicia workers que fazem "Batching" (agrupamento de dados).
func StartWorkerPool(ctx context.Context, numWorkers int, batchSize int, timeout time.Duration, messages <-chan amqp.Delivery, handler BatchHandler) *sync.WaitGroup {
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, i, batchSize, timeout, messages, handler, &wg)
	}

	log.Printf("Pool de Batching iniciado. %d workers, Batch Size: %d", numWorkers, batchSize)
	return &wg
}

func worker(ctx context.Context, id int, batchSize int, timeout time.Duration, messages <-chan amqp.Delivery, handler BatchHandler, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("[Worker %d] pronto para processar lotes.", id)

	batch := make([]amqp.Delivery, 0, batchSize)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		// Envia para salvar na base de dados o lote atual
		err := handler(ctx, batch)
		if err != nil {
			log.Printf("[Worker %d] Erro ao salvar lote de %d itens: %v", id, len(batch), err)
			// Em caso de falha severa, você pode aplicar um Nack.
			// Como é um pool compartilhado, vamos evitar Nack(Multiple=true) para segurança.
			for _, msg := range batch {
				_ = msg.Nack(false, true) // Re-enfileira os itens p/ tentativa futura
			}
		} else {
			// Sucesso! Confirma (ACK) pro RabbitMQ remover da fila definitivamente.
			for _, msg := range batch {
				_ = msg.Ack(false)
			}
			log.Printf("[Worker %d] Sucesso ao gravar lote com %d itens no BD.", id, len(batch))
		}

		// Limpa o lote para o próximo ciclo
		batch = make([]amqp.Delivery, 0, batchSize)
	}

	for {
		select {
		case <-ctx.Done(): // Sistema está desligando
			flush() // Salva o que sobrou no buffer correndo
			log.Printf("[Worker %d] desligado com segurança.", id)
			return

		case <-ticker.C: // Estourou tempo limite (Ex: se faz 3 segundos e tem 3 mensagens na mão, salva logo)
			flush()

		case msg, ok := <-messages: // Chegou voto novo
			if !ok {
				flush()
				log.Printf("[Worker %d] canal fechado, encerrando.", id)
				return
			}

			batch = append(batch, msg)

			// Se o lote encheu, dispara pro banco imediatamente!
			if len(batch) >= batchSize {
				flush()
				// Reseta o temporizador pra evitar chamadas duplas desnecessárias
				ticker.Reset(timeout)
			}
		}
	}
}
