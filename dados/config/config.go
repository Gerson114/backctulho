package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// AppConfig armazena os parâmetros de inicialização
type AppConfig struct {
	RabbitURL    string
	QueueName    string
	NumWorkers   int
	BatchSize    int
	Timeout      time.Duration
	DBDSN        string // PostgreSQL DSN
	PrefetchCount int   // number of messages to prefetch from RabbitMQ
}

// LoadConfig carrega as variáveis do arquivo .env e ambiente
func LoadConfig() AppConfig {
	// Carrega o arquivo .env se ele existir
	err := godotenv.Load()
	if err != nil {
		log.Println("[Aviso] Arquivo .env não encontrado, usando variáveis de ambiente.")
	}

	return AppConfig{
		RabbitURL:  getEnv("RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
		QueueName:  getEnv("QUEUE_NAME", "minha_fila"),
		NumWorkers: getEnvAsInt("NUM_WORKERS", 20),
		BatchSize:  getEnvAsInt("BATCH_SIZE", 500),
		Timeout:    getEnvAsDuration("BATCH_TIMEOUT", 500*time.Millisecond),
		PrefetchCount: getEnvAsInt("PREFETCH_COUNT", 20000),
		DBDSN:      getEnv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/dbname?sslmode=disable"),
	}
}

// funçôes auxiliares para facilitar a leitura profissional
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
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
	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}
	return fallback
}
