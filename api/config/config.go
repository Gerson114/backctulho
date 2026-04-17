package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	RabbitURL     string
	RedisURL      string
	Port          string
	GinMode       string
	AllowedOrigin string // NOVO: Para restringir o CORS
	PostgresDSN   string // NOVO: Endpoint do banco de dados
	PrefetchCount int
	BatchSize     int
}

func LoadConfig() *Config {
	// Carrega .env se existir (útil em dev), em prod usa variáveis do sistema
	_ = godotenv.Load()

	cfg := &Config{
		RabbitURL:     strings.TrimSpace(os.Getenv("RABBITMQ_URL")),
		RedisURL:      strings.TrimSpace(os.Getenv("REDIS_VALIDACAO_URL")),
		Port:          getEnv("PORT", "8080"),
		GinMode:       getEnv("GIN_MODE", "release"),
		AllowedOrigin: strings.TrimSpace(getEnv("ALLOWED_ORIGIN", "")),
		PostgresDSN:   strings.TrimSpace(getEnv("POSTGRES_DSN", "")),
	}

	// Validação Crítica: Em produção, o sistema NÃO deve subir sem as URLs externas e origin
	if cfg.GinMode == "release" && cfg.AllowedOrigin == "" {
		log.Println("⚠️ AVISO: ALLOWED_ORIGIN não definido em modo release. O CORS pode bloquear todas as requisições legítimas.")
	}
	if cfg.GinMode == "release" && cfg.AllowedOrigin == "*" {
		log.Println("🚨 PERIGO: ALLOWED_ORIGIN definido como '*' em produção. Isso é inseguro!")
	}

	if cfg.RabbitURL == "" {
		log.Fatal("❌ ERRO: RABBITMQ_URL não definida. O sistema não pode rodar em produção sem fila.")
	}
	if cfg.RedisURL == "" {
		log.Fatal("❌ ERRO: REDIS_URL não definida. A validação de duplicidade exige Redis.")
	}
	if cfg.PostgresDSN == "" {
		log.Fatal("❌ ERRO: POSTGRES_DSN não definida. O endpoint do banco é obrigatório.")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
