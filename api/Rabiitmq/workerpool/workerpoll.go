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
func (wp *WorkerPool) Publicar(voto models.Voto) error {
	select {
	case wp.fila <- voto:
		return nil
	default:
		return fmt.Errorf("workerpool: fila cheia, sistema sobrecarregado")
	}
}

// EstaSobrecarregado retorna true se a fila interna estiver com mais de 90% de ocupação.
// Útil para o Health Check de produção sinalizar ao Load Balancer para reduzir o tráfego.
func (wp *WorkerPool) EstaSobrecarregado() bool {
	capacidade := cap(wp.fila)
	uso := len(wp.fila)

	if capacidade == 0 {
		return false
	}

	// Alerta quando atingir 90% da capacidade do buffer interno
	return float64(uso)/float64(capacidade) > 0.9
}
