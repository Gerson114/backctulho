package main

import (
	"log"

	"nome-do-projeto/app"
	"nome-do-projeto/config"
)

func main() {
	// Carrega todas as variáveis (.env, rabbit, vars de workers) de forma enxuta
	cfg := config.LoadConfig()

	// Inicia a aplicação usando as configs blindadas
	if err := app.Run(cfg); err != nil {
		log.Fatalf("Erro fatal de execução: %v", err)
	}
}
