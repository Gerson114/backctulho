package db

import (
    "fmt"
    "log"
    "time"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

// InitDB initializes a GORM DB connection with connection pooling and prepared statements.
func InitDB(dsn string) (*gorm.DB, error) {
    // High‑performance GORM configuration
    cfg := &gorm.Config{
        Logger:      logger.Default.LogMode(logger.Silent), // silence per‑query logs
        PrepareStmt: true,
    }

    db, err := gorm.Open(postgres.Open(dsn), cfg)
    if err != nil {
        return nil, fmt.Errorf("erro ao conectar ao Postgres: %w", err)
    }

    sqlDB, err := db.DB()
    if err != nil {
        return nil, err
    }

    // Connection pool settings (tuned for workers)
    sqlDB.SetMaxOpenConns(100)
    sqlDB.SetMaxIdleConns(50)
    sqlDB.SetConnMaxLifetime(time.Hour)

    Instance = db
    log.Println("[DB] Pool de conexões Postgres inicializado.")
    return db, nil
}

// Instance é a instância global do GORM para ser usada pelos handlers
var Instance *gorm.DB
