package workerpool

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"meu-projeto/models"
)

const nomeFila = "votos"

// WorkerPool gerencia N goroutines, cada uma com seu próprio canal RabbitMQ.
// Resolve o problema de concorrência: amqp.Channel não é thread-safe.
type WorkerPool struct {
	conn    *amqp.Connection // conexão compartilhada (thread-safe)
	fila    chan models.Voto // fila interna em memória entre a API e os workers
	workers int             // quantidade de goroutines rodando em paralelo
}

// Novo cria o WorkerPool e inicia os workers em background.
//
//   - conn:    conexão RabbitMQ aberta no main.go
//   - workers: quantidade de goroutines (recomendado: 20)
//   - buffer:  capacidade máxima da fila interna (recomendado: 10000)
func Novo(conn *amqp.Connection, workers int, buffer int) *WorkerPool {
	wp := &WorkerPool{
		conn:    conn,
		fila:    make(chan models.Voto, buffer),
		workers: workers,
	}
	wp.iniciar()
	return wp
}

// iniciar dispara os N workers como goroutines independentes.
func (wp *WorkerPool) iniciar() {
	for i := 0; i < wp.workers; i++ {
		go wp.worker(i)
	}
}

// Publicar é chamado pela API para cada voto recebido.
// Retorna em nanosegundos — não espera o RabbitMQ confirmar.
// Se a fila interna estiver cheia, aplica back-pressure retornando erro.
func (wp *WorkerPool) Publicar(voto models.Voto) error {
	select {
	case wp.fila <- voto:
		return nil
	default:
		return fmt.Errorf("workerpool: fila cheia, sistema sobrecarregado")
	}
}
