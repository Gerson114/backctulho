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
	// 1. Criação de contexto com Graceful Shutdown via Sinais de SO
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupGracefulShutdown(cancel)

	// 2. Conectando ao Servidor RabbitMQ
	log.Println("[App] Inicializando conexão com RabbitMQ...")
	conn, ch, err := connect.ConnectRabbitMQ(cfg.RabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()

	// 3. Inicializaçao do Consumer e Worker Pool mapeando para o Handler isolado
	wg, err := consumer.StartConsumer(
		ctx,
		ch,
		cfg.QueueName,
		cfg.NumWorkers,
		cfg.BatchSize,
		cfg.Timeout,
		handlers.ProcessVoteBatch,
	)
	if err != nil {
		return err
	}

	log.Println("[App] Sistema em execução. Pressione CTRL+C para sair.")

	// Espera o sinal de encerramento do sistema disparar o cancelamento do ctx
	<-ctx.Done()

	waitGracefulShutdown(wg)
	return nil
}

// setupGracefulShutdown lida com Ctrl+C e eventos de Docker Stop p/ desligamento seguro
func setupGracefulShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n[Sistema] Recebido sinal, iniciando desligamento seguro...")
		cancel()
	}()
}

// waitGracefulShutdown segura a execução principal enquanto os workers finalizam até o limite de tempo
func waitGracefulShutdown(wg interface{ Wait() }) {
	log.Println("[Sistema] Aguardando workers finalizarem tarefas em andamento...")
	waitCh := make(chan struct{})

	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		log.Println("[Sistema] Todos os dados salvos com sucesso. Encerrando.")
	case <-time.After(10 * time.Second):
		log.Println("[AVISO] Timeout de 10s alcançado com workers lentos. Forçando encerramento.")
	}
}
