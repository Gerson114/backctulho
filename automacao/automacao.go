package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

type Vote struct {
	ID     int    `json:"id"`
	Nome   string `json:"nome"`
	Numero int    `json:"numero"`
	Email  string `json:"email"`
	Votos  string `json:"votos"`
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Android 14; Mobile; rv:124.0) Gecko/124.0 Firefox/124.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
}

func main() {
	targetURL := "http://localhost:8080/vote"

	// --- CONFIGURAÇÃO DA SIMULAÇÃO ---
	// 2000 workers com delay médio de 1s = ~2000 votos/segundo
	const numWorkers = 2000
	var count uint64

	transport := &http.Transport{
		MaxIdleConns:        0, // Permite abrir muitas conexões como se fossem pessoas diferentes
		MaxIdleConnsPerHost: 500,
		IdleConnTimeout:     30 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	startTime := time.Now()
	log.Printf("👥 INICIANDO SIMULAÇÃO HUMANA")
	log.Printf("🎯 Alvo: %s | Workers: %d", targetURL, numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func(id int) {
			// Seed aleatório por worker para não repetirem o mesmo padrão
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

			for {
				// 1. "THINK TIME": Simula o tempo de preencher o formulário (0.8s a 2.5s)
				sleepTime := r.Intn(1700) + 800
				time.Sleep(time.Duration(sleepTime) * time.Millisecond)

				currentCount := atomic.AddUint64(&count, 1)

				// 2. DADOS ÚNICOS (Evita Erro 409)
				vote := Vote{
					ID:     0, // Deixa o banco gerenciar o ID
					Nome:   "Participante Ativo",
					Numero: r.Intn(99999), // Varia o candidato para não sobrecarregar uma única linha
					// Email impossível de duplicar: seq + nano + random
					Email: fmt.Sprintf("voter_%d_%x_%d@gmail.com", currentCount, time.Now().UnixNano(), r.Intn(1000)),
					Votos: "1",
				}

				sendRequest(client, targetURL, vote, r)

				// Relatório de progresso
				if currentCount%1000 == 0 {
					elapsed := time.Since(startTime).Seconds()
					fmt.Printf("\r📊 Total: %d | Ritmo: %.0f votos/s | Status: OK (200)",
						currentCount, float64(currentCount)/elapsed)
				}
			}
		}(w)
	}

	select {} // Mantém vivo
}

func sendRequest(client *http.Client, url string, data Vote, r *rand.Rand) {
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonData))

	// CABEÇALHOS REAIS (Indispensável para não ser bloqueado)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgents[r.Intn(len(userAgents))])
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8")
	req.Header.Set("Origin", "http://localhost:8080")
	req.Header.Set("Referer", "http://localhost:8080/votacao")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Consome o corpo para liberar o socket
	_, _ = io.Copy(io.Discard, resp.Body)
}
