package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nome-do-projeto/config"
	"nome-do-projeto/handlers"
	"nome-do-projeto/rabbitmq/connect"
	"nome-do-projeto/rabbitmq/consumer"
)

// Run inicia o ciclo de vida completo da aplicação de consumo.
func Run(cfg config.AppConfig) error {
	// 1. Contexto com Graceful Shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupGracefulShutdown(cancel)

	// 2. Conexão TCP única com RabbitMQ (os canais são abertos internamente)
	log.Println("[App] Conectando ao RabbitMQ...")
	conn, err := connect.ConnectRabbitMQ(cfg.RabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 3. Inicia pipeline: N collectors (canais AMQP) → batchCh → M processors
	wg, err := consumer.StartConsumer(
		ctx,
		conn,
		cfg.QueueName,
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

	log.Println("[App] ✅ Sistema em execução. Pressione CTRL+C para sair.")
	<-ctx.Done()

	waitGracefulShutdown(wg)
	return nil
}

func setupGracefulShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\n[Sistema] Sinal recebido, iniciando desligamento seguro...")
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
		log.Println("[AVISO] Timeout de 15s: encerrando à força.")
	}
}
