package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"meu-projeto/Rabiitmq/conexao"
	"meu-projeto/Rabiitmq/workerpool"
	"meu-projeto/router"
	"meu-projeto/validacao"
)

func main() {
	// Carrega variáveis do .env (só em dev — em produção usa env vars reais)
	if err := godotenv.Load(); err != nil {
		log.Println("Arquivo .env não encontrado, usando variáveis de ambiente do sistema")
	}

	// 1. Conecta ao RabbitMQ (URL via RABBITMQ_URL env var)
	conn := conexao.ConexaoRabbitmq()
	defer conn.Close()

	// 2. Inicia 20 workers — cada um com seu canal RabbitMQ próprio
	wp := workerpool.Novo(conn, 20, 10_000)

	// 3. Conecta ao Redis (anti-duplicata) via URL
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(redisUrl)
	if err != nil {
		log.Fatalf("Erro ao configurar Redis URL: %s", err)
	}
	rdb := redis.NewClient(opts)
	val := validacao.Novo(rdb)

	// 4. Sobe o servidor HTTP em goroutine separada
	r := router.Iniciar(wp, val) // Passa o workerpool e o validador
	porta := os.Getenv("PORT")
	if porta == "" {
		porta = "8080"
	}

	go func() {
		log.Printf("API rodando em :%s", porta)
		if err := r.Run(":" + porta); err != nil {
			log.Fatalf("erro ao iniciar servidor: %s", err)
		}
	}()

	// 5. Graceful shutdown — aguarda Ctrl+C ou SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("Sinal recebido, encerrando servidor...")
}
