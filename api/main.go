package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"meu-projeto/Rabiitmq/conexao"
	"meu-projeto/Rabiitmq/workerpool"
	"meu-projeto/config"
	"meu-projeto/router"
	"meu-projeto/validacao"
)

func main() {
	// 1. Carrega Configurações (Centralizado e validado para Produção)
	cfg := config.LoadConfig()

	// Configura Modo de Produção do Gin
	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 2. Conecta ao RabbitMQ (Usa a URL do ambiente sem fallback local)
	conn := conexao.ConexaoRabbitmq(cfg.RabbitURL)
	defer conn.Close()

	// 3. Inicia Pool de Workers Internos (Producer)
	wp := workerpool.Novo(conn, 30, 10_000)

	// 4. Conecta ao Redis com Pool otimizado
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("❌ Erro fatal: falha ao parsear REDIS_URL: %v", err)
	}

	opts.PoolSize = 100
	opts.MinIdleConns = 20

	rdb := redis.NewClient(opts)
	val := validacao.Novo(rdb)

	// 5. Inicia o Servidor HTTP (Gin)
	r := router.Iniciar(wp, val, cfg)

	go func() {
		log.Printf("🚀 SERVIDOR EM PRODUÇÃO | Porta: %s | Modo: %s", cfg.Port, cfg.GinMode)
		if err := r.Run(":" + cfg.Port); err != nil {
			log.Fatalf("❌ Erro fatal no servidor: %v", err)
		}
	}()

	// Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("🛑 Sinal de encerramento recebido. Finalizando serviços...")
}
