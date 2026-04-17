package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"contagem/config"
	"contagem/rabbitmq/conexao"
	"contagem/rabbitmq/consumer"
	"contagem/redis"
	"contagem/router"
)

func main() {
	// 1. Config
	cfg := config.LoadConfig()

	// 2. Redis
	rdb := redis.Conectar(cfg.RedisURL)
	defer rdb.Fechar()

	// 3. RabbitMQ
	conn := conexao.Conectar(cfg.RabbitURL)
	defer conn.Close()

	// 4. Iniciar Servidor de Resultados (Porta 8081)
	// Este servidor entrega os votos para o Frontend
	go func() {
		r := router.Iniciar(rdb)
		log.Println("🚀 Servidor de RESULTADOS ativo na porta 8081")
		if err := r.Run(":8081"); err != nil {
			log.Fatalf("❌ Falha ao iniciar servidor de resultados: %v", err)
		}
	}()

	// 5. Iniciar Consumer (em background via loop interno)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		consumer.Iniciar(conn, cfg.QueueName, cfg.ExchangeName, rdb)
	}()

	log.Println("✅ Serviço de Contagem e Resultados em execução. Pressione CTRL+C para sair.")
	<-stopChan
	log.Println("🛑 Encerrando serviço...")
}
