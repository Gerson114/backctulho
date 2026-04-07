package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// Vote define a estrutura do payload solicitado
type Vote struct {
	ID     int    `json:"id"`
	Nome   string `json:"nome"`
	Numero int    `json:"numero"`
	Email  string `json:"email"`
	Votos  string `json:"votos"`
}

func main() {
	targetURL := "http://localhost:8080/vote"
	requestsPerSecond := 2000
	
	// Configuração profissional do cliente HTTP para alta carga
	// Sem isso, o Windows "morre" por falta de portas TCP em segundos
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          2000,
		MaxIdleConnsPerHost:   2000, // IMPORTANTE: Reusa conexões para os 2000 req/s
		IdleConnTimeout:       90 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	log.Printf("Iniciando automação: %d votos por segundo em %s", requestsPerSecond, targetURL)

	ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
	defer ticker.Stop()

	var wg sync.WaitGroup
	count := 0
	startTime := time.Now()

	for range ticker.C {
		wg.Add(1)
		
		// Incrementamos os valores para garantir que cada voto seja ÚNICO
		count++
		currentID := count
		uniqueNumero := 1000000 + count // Ex: 1000001, 1000002...
		uniqueEmail := fmt.Sprintf("eleitor_%d@teste.com", count)

		go func(id, num int, email string) {
			defer wg.Done()
			
			payload := Vote{
				ID:     id,
				Nome:   "Gerson",
				Numero: num,
				Email:  email,
				Votos:  "5",
			}

			jsonData, err := json.Marshal(payload)
			if err != nil {
				return
			}

			resp, err := client.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				return
			}
			resp.Body.Close()
		}(currentID, uniqueNumero, uniqueEmail)

		// Exibe progresso a cada segundo
		if count%requestsPerSecond == 0 {
			elapsed := time.Since(startTime).Seconds()
			fmt.Printf("\r[Progresso] %d votos únicos enviados em %.1fs", count, elapsed)
		}
	}

	wg.Wait()
}
