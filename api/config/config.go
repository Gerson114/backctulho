package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RabbitURL     string
	RedisURL      string
	Port          string
	GinMode       string
	AllowedOrigin string // NOVO: Para restringir o CORS
	PrefetchCount int
	BatchSize     int
}

func LoadConfig() *Config {
	// Carrega .env se existir (útil em dev), em prod usa variáveis do sistema
	_ = godotenv.Load()

	cfg := &Config{
		RabbitURL:     os.Getenv("RABBITMQ_URL"),
		RedisURL:      os.Getenv("REDIS_URL"),
		Port:          getEnv("PORT", "8080"),
		GinMode:       getEnv("GIN_MODE", "release"),
		AllowedOrigin: getEnv("ALLOWED_ORIGIN", "*"), // URL do seu site oficial
	}

	// Validação Crítica: Em produção, o sistema NÃO deve subir sem as URLs externas
	if cfg.RabbitURL == "" {
		log.Fatal("❌ ERRO: RABBITMQ_URL não definida. O sistema não pode rodar em produção sem fila.")
	}
	if cfg.RedisURL == "" {
		log.Fatal("❌ ERRO: REDIS_URL não definida. A validação de duplicidade exige Redis.")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
