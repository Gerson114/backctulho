package workerpool

import (
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"meu-projeto/models"
)

const nomeExchange = "votos_exchange"

// WorkerPool gerencia N goroutines que compartilham um pool pequeno de canais RabbitMQ.
// Resolve o problema de limites de canais (CloudAMQP) mantendo alta vazão.
type WorkerPool struct {
	conn    *amqp.Connection // conexão compartilhada (thread-safe)
	fila    chan models.Voto // fila interna em memória
	workers int              // quantidade de goroutines rodando em paralelo
	canais  []*amqp.Channel  // pool fixo de canais (limite de 10)
	canalMu sync.Mutex       // protege o acesso aos canais para publicação
	indice  int              // para rotacionar o uso dos canais
}

// Novo cria o WorkerPool e inicia os workers em background com 10 canais compartilhados.
func Novo(conn *amqp.Connection, workers int, buffer int) *WorkerPool {
	wp := &WorkerPool{
		conn:    conn,
		fila:    make(chan models.Voto, buffer),
		workers: workers,
		canais:  make([]*amqp.Channel, 0),
	}

	// Pré-inicializa 10 canais estáveis (dentro do limite de 20 do CloudAMQP)
	for i := 0; i < 10; i++ {
		ch, err := conn.Channel()
		if err != nil {
			log.Printf("⚠️ WorkerPool: falha ao abrir canal do pool: %v", err)
			continue
		}

		// Declara Exchange uma única vez para todos
		_ = ch.ExchangeDeclare(nomeExchange, "fanout", true, false, false, false, nil)
		wp.canais = append(wp.canais, ch)
	}

	if len(wp.canais) == 0 {
		log.Fatal("❌ WorkerPool: nenhum canal RabbitMQ pôde ser aberto")
	}

	log.Printf("📥 WorkerPool: Inicializado com %d workers compartilhando %d canais", workers, len(wp.canais))
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
