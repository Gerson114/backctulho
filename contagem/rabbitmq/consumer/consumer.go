package consumer

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"contagem/redis"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Voto struct {
	EmendaVotada string `json:"emenda_votada"`
}

var (
	buffer      = make(map[string]int64)
	bufferMutex sync.Mutex

	// 🚀 CACHE GLOBAL EM RAM: Placar em tempo real para o Front-end
	SnapshotCache   = make(map[string]int64)
	SnapshotMutex   sync.RWMutex // RWMutex é mais eficiente para muitas leituras
)

func Iniciar(conn *amqp.Connection, queueName, exchangeName string, redisClient *redis.Client) {
	for {
		log.Println("📥 Preparando canal de consumo RabbitMQ...")
		ch, err := conn.Channel()
		if err != nil {
			log.Printf("❌ Falha ao abrir canal: %v. Tentando novamente em 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Declara Exchange (Fanout)
		_ = ch.ExchangeDeclare(exchangeName, "fanout", true, false, false, false, nil)

		// Declara Fila
		q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
		if err != nil {
			log.Printf("❌ Falha ao declarar fila: %v. Reiniciando...", err)
			ch.Close()
			time.Sleep(2 * time.Second)
			continue
		}

		// Bind
		_ = ch.QueueBind(q.Name, "", exchangeName, false, nil)

		msgs, err := ch.Consume(q.Name, "contagem-consumer", true, false, false, false, nil)
		if err != nil {
			log.Printf("❌ Falha ao iniciar consumo: %v. Reiniciando...", err)
			ch.Close()
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("🚀 ESCUTA ATIVA: Agardando votos na fila: %s", q.Name)

		// Inicia o Flush Periódico (Batching) único se ainda não estiver rodando
		once := sync.Once{}
		once.Do(func() {
			go periodicFlush(redisClient)
		})

		var votosContadosNoCiclo int64
		for d := range msgs {
			var v Voto
			if err := json.Unmarshal(d.Body, &v); err != nil {
				continue
			}

			// Acumula no buffer
			bufferMutex.Lock()
			buffer[v.EmendaVotada]++
			bufferMutex.Unlock()

			votosContadosNoCiclo++
			if votosContadosNoCiclo%500 == 0 {
				log.Printf("📊 BI: %d votos processados nas últimas mensagens", votosContadosNoCiclo)
			}
		}

		log.Println("⚠️ Canal RabbitMQ fechado. Tentando reconectar...")
		ch.Close()
		time.Sleep(2 * time.Second)
	}
}

func periodicFlush(redisClient *redis.Client) {
	ticker := time.NewTicker(500 * time.Millisecond) // Atualiza 2 vezes por segundo
	defer ticker.Stop()

	for range ticker.C {
		bufferMutex.Lock()
		if len(buffer) == 0 {
			bufferMutex.Unlock()
			continue
		}

		// Copia e limpa o buffer
		lote := make(map[string]int64)
		var totalLote int64
		for id, count := range buffer {
			lote[id] = count
			totalLote += count
			delete(buffer, id)
		}
		bufferMutex.Unlock()

		// 1. Persistência em Lote no Redis (Atomic)
		ctx := context.Background()
		for id, count := range lote {
			// Incrementa o individual
			_ = redisClient.IncrementarLoteVotos(ctx, id, count)
		}

		// 2. Busca o ESTADO REAL de todos os votos para Sincronização Total (Self-Healing)
		mapaCompleto, err := redisClient.GetTodosVotosEmendas(ctx)
		if err != nil {
			log.Printf("⚠️ Erro ao buscar mapa de votos para sincronização: %v", err)
			continue
		}

		// 🛡️ ATUALIZA CACHE EM RAM (Fim do gargalo no Redis)
		SnapshotMutex.Lock()
		SnapshotCache = mapaCompleto
		SnapshotMutex.Unlock()

		// 3. Publicação do Snapshot de Estado em Tempo Real
		_ = redisClient.PublicarVotos(ctx, "votos_realtime", mapaCompleto)
	}
}
