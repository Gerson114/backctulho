package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	RabbitURL    string
	RedisURL     string
	QueueName    string
	ExchangeName string
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		RabbitURL:    strings.TrimSpace(os.Getenv("RABBITMQ_URL")),
		RedisURL:     strings.TrimSpace(os.Getenv("REDIS_CONTAGEM_URL")),
		QueueName:    strings.TrimSpace(os.Getenv("QUEUE_NAME")),
		ExchangeName: strings.TrimSpace(os.Getenv("EXCHANGE_NAME")),
	}

	if cfg.RabbitURL == "" || cfg.RedisURL == "" {
		log.Fatalf("❌ ERRO: RABBITMQ_URL ou REDIS_URL não definidas no .env")
	}

	return cfg
}
