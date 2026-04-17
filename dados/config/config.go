package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type AppConfig struct {
	RabbitURL     string
	QueueName     string
	NumCollectors int
	NumProcessors int
	BatchSize     int
	Timeout       time.Duration
	PostgresDSN   string
	PrefetchCount int
	ExchangeName  string
}

func LoadConfig() AppConfig {
	// Força o carregamento do .env do diretório atual
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("[Config] ⚠️  Aviso: Erro ao carregar .env: %v. Usando env vars do sistema.", err)
	} else {
		log.Println("[Config] ✅ Arquivo .env carregado com sucesso.")
	}

	cfg := AppConfig{
		RabbitURL:     getEnv("RABBITMQ_URL", ""),
		QueueName:     getEnv("QUEUE_NAME", "votos"),
		NumCollectors: getEnvAsInt("NUM_COLLECTORS", 3),
		NumProcessors: getEnvAsInt("NUM_PROCESSORS", 60),
		BatchSize:     getEnvAsInt("BATCH_SIZE", 1000),
		Timeout:       getEnvAsDuration("BATCH_TIMEOUT", 200*time.Millisecond),
		PrefetchCount: getEnvAsInt("PREFETCH_COUNT", 10000),
		PostgresDSN:   getEnv("POSTGRES_DSN", ""),
		ExchangeName:  getEnv("EXCHANGE_NAME", "votos_exchange"),
	}

	// Validação de Produção
	if cfg.RabbitURL == "" {
		log.Fatal("❌ ERRO: RABBITMQ_URL de produção não definida nos dados.")
	}
	if cfg.PostgresDSN == "" {
		log.Fatal("❌ ERRO: POSTGRES_DSN de produção não definida nos dados.")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if d, err := time.ParseDuration(valueStr); err == nil {
		return d
	}
	return fallback
}
