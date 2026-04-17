package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nome-do-projeto/config"
	"nome-do-projeto/db"
	"nome-do-projeto/handlers"
	"nome-do-projeto/models"
	"nome-do-projeto/rabbitmq/connect"
	"nome-do-projeto/rabbitmq/consumer"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Run inicia o ciclo de vida completo da aplicação com Reconexão Automática.
func Run(cfg config.AppConfig) error {
	// 0. Inicializa Banco de Dados (Uma vez só)
	log.Println("[App] Conectando ao Postgres...")
	database, err := db.InitDB(cfg.PostgresDSN)
	if err != nil {
		return err
	}
	
	if err := database.AutoMigrate(&models.Voto{}); err != nil {
		log.Printf("[App] Erro na migração: %v", err)
	}

	// 1. Contexto Global de Desligamento
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupGracefulShutdown(cancel)

	log.Println("[App] ✅ Ciclo de vida iniciado. Puxando votos...")

	// ── LOOP DE RESILIÊNCIA (Self-Healing) ──────────────────────────────────────
	for {
		select {
		case <-ctx.Done():
			log.Println("[App] Encerrando por solicitação do sistema.")
			return nil
		default:
			// Tenta conectar e processar
			err := startConsumptionCycle(ctx, cfg)
			if err != nil {
				log.Printf("[App] ⚠️ Falha na conexão/consumo: %v. Tentando reconectar em 5s...", err)
				time.Sleep(5 * time.Second)
				continue
			}
		}
	}
}

// startConsumptionCycle gerencia uma única sessão de conexão e consumo.
func startConsumptionCycle(ctx context.Context, cfg config.AppConfig) error {
	log.Println("[App] 🔌 Conectando ao RabbitMQ...")
	conn, err := connect.ConnectRabbitMQ(cfg.RabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Canal para detectar se a conexão caiu
	notifyClose := conn.NotifyClose(make(chan *amqp.Error))

	// Inicia os workers
	wg, err := consumer.StartConsumer(
		ctx,
		conn,
		cfg.QueueName,
		cfg.ExchangeName,
		cfg.PrefetchCount,
		cfg.NumCollectors,
		cfg.NumProcessors,
		cfg.BatchSize,
		cfg.Timeout,
		handlers.ProcessVoteBatch,
	)
	if err != nil {
		return err
	}

	log.Println("[App] 🚀 Pipeline ativo e consumindo.")

	// Aguarda um dos 3 eventos:
	select {
	case <-ctx.Done():
		log.Println("[App] Desligamento solicitado (SIGINT/SIGTERM)")
		waitGracefulShutdown(wg)
		return nil
	case amqpErr := <-notifyClose:
		log.Printf("[App] ❌ Conexão RabbitMQ perdida: %v", amqpErr)
		return amqpErr
	}
}

func setupGracefulShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\n[Sistema] Sinal recebido (CTRL+C).")
		cancel()
	}()
}

func waitGracefulShutdown(wg interface{ Wait() }) {
	log.Println("[Sistema] Aguardando workers finalizarem...")
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
		log.Println("[Sistema] ✅ Todos os dados salvos. Encerrando.")
	case <-time.After(15 * time.Second):
		log.Println("[AVISO] Timeout de encerrando à força.")
	}
}
