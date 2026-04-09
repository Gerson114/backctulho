package db

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Instance é a instância global do banco para acesso simplificado se necessário
var Instance *gorm.DB

// Init estabelece a conexão e configura o pool de conexões
func Init(dsn string) (*gorm.DB, error) {
	// Configuração do GORM para alta performance
	config := &gorm.Config{
		// O modo Silent evita que cada INSERT gere um log, poupando CPU e disco
		Logger: logger.Default.LogMode(logger.Silent),
		// Prepara statements para ganhar velocidade em inserções repetitivas
		PrepareStmt: true,
	}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar no Postgres: %w", err)
	}

	// Acessa a base SQL genérica para configurar o Pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// --- CONFIGURAÇÕES DO POOL (VITAL PARA OS WORKERS) ---

	// Máximo de conexões abertas (Ideal: um pouco mais que o número de Workers)
	sqlDB.SetMaxOpenConns(100)

	// Máximo de conexões inativas no pool esperando trabalho
	// Aumentado para 50 para suportar a alta concorrência de 60+ workers
	sqlDB.SetMaxIdleConns(50)

	// Tempo máximo que uma conexão pode ser reutilizada
	sqlDB.SetConnMaxLifetime(time.Hour)

	Instance = db
	log.Println("[DB] Pool de conexões Postgres inicializado.")

	return db, nil
}
