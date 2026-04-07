package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	alvo    = "http://localhost:8080/vote"
	reqPorS = 1000           // requisições por segundo
	duracao = 30 * time.Second // duração do teste
)

type Voto struct {
	ID           int64  `json:"id"`
	Nome         string `json:"nome"`
	Numero       int    `json:"numero"`
	EmendaVotada string `json:"emenda_votada"`
}

func main() {
	var (
		total   int64
		ok202   int64
		err503  int64
		errRede int64
	)

	// Pool de clientes HTTP reutilizáveis (evita abrir nova conn a cada req)
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 500,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	intervalo := time.Second / time.Duration(reqPorS) // ~1ms entre cada req
	fim := time.Now().Add(duracao)

	log.Printf("🚀 Automação iniciada: %d req/s por %s → %s", reqPorS, duracao, alvo)

	// Estatísticas a cada 1 segundo
	go func() {
		tick := time.NewTicker(1 * time.Second)
		defer tick.Stop()
		anterior := int64(0)
		for range tick.C {
			atual := atomic.LoadInt64(&total)
			log.Printf("📊 %d req/s | total=%d | 202=%d | 503=%d | erro=%d",
				atual-anterior,
				atual,
				atomic.LoadInt64(&ok202),
				atomic.LoadInt64(&err503),
				atomic.LoadInt64(&errRede),
			)
			anterior = atual
			if time.Now().After(fim) {
				return
			}
		}
	}()

	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()

	var wg sync.WaitGroup
	id := int64(0)

	for time.Now().Before(fim) {
		<-ticker.C
		wg.Add(1)
		go func(votoID int64) {
			defer wg.Done()
			enviar(client, votoID, &total, &ok202, &err503, &errRede)
		}(id)
		id++
	}

	wg.Wait()

	elapsed := duracao
	log.Println("══════════════════════════════════════════")
	log.Printf("✅  RESULTADO FINAL")
	log.Printf("    Duração          : %s", elapsed)
	log.Printf("    Total enviados   : %d", atomic.LoadInt64(&total))
	log.Printf("    Média req/s      : %.0f", float64(atomic.LoadInt64(&total))/duracao.Seconds())
	log.Printf("    202 Aceitos      : %d", atomic.LoadInt64(&ok202))
	log.Printf("    503 Sobrecarga   : %d", atomic.LoadInt64(&err503))
	log.Printf("    Erros de rede    : %d", atomic.LoadInt64(&errRede))
	log.Println("══════════════════════════════════════════")
}

func enviar(client *http.Client, id int64, total, ok202, err503, errRede *int64) {
	voto := Voto{
		ID:           id,
		Nome:         fmt.Sprintf("Candidato %d", id%10),
		Numero:       int(id%5) + 1,
		EmendaVotada: fmt.Sprintf("E%d", id%3+1),
	}

	corpo, _ := json.Marshal(voto)
	resp, err := client.Post(alvo, "application/json", bytes.NewReader(corpo))
	atomic.AddInt64(total, 1)

	if err != nil {
		atomic.AddInt64(errRede, 1)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 202:
		atomic.AddInt64(ok202, 1)
	case 503:
		atomic.AddInt64(err503, 1)
	}
}
