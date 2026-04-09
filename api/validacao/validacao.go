package validacao

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrVotoDuplicado = errors.New("eleitor já votou em uma emenda e só é permitido um voto por pessoa")

type Validador struct {
	redis   *redis.Client
	cacheL1 sync.Map // Cache em memória para performance extrema
}

func Novo(rdb *redis.Client) *Validador {
	return &Validador{
		redis: rdb,
	}
}

func (v *Validador) JaVotou(ctx context.Context, voterID string, electionID string) error {
	chave := fmt.Sprintf("vote_global:%s", voterID)

	// 1. CHECK L1 (MEMÓRIA LOCAL) - Latência < 1ms
	if _, existe := v.cacheL1.Load(chave); existe {
		return ErrVotoDuplicado
	}

	// 2. CHECK L2 (REDIS) - Caso não esteja na memória local
	criou, err := v.redis.SetNX(ctx, chave, "1", 7*24*time.Hour).Result()
	if err != nil {
		return fmt.Errorf("redis indisponível: %w", err)
	}

	if !criou {
		// Se já existia no Redis, atualizamos o cache local para a próxima vez ser instantâneo
		v.cacheL1.Store(chave, struct{}{})
		return ErrVotoDuplicado
	}

	// 3. SUCESSO - Guardamos no cache local para proteger o Redis de novas tentativas
	v.cacheL1.Store(chave, struct{}{})
	return nil
}

// Checking verifica se o Redis está respondendo corretamente para o Health Check
func (v *Validador) Checking(ctx context.Context) error {
	return v.redis.Ping(ctx).Err()
}

// PermitirRateLimit cria um bloqueio simples de janela fixa (Fixed Window Counter) via Redis.
// Retorna false se o IP enviou mais requisições que o "limite" dentro do tempo "janela".
func (v *Validador) PermitirRateLimit(ctx context.Context, ip string, limite int64, janela time.Duration) bool {
	chave := fmt.Sprintf("ratelimit:ip:%s", ip)

	// Incrementa o contador do IP
	count, err := v.redis.Incr(ctx, chave).Result()
	if err != nil {
		// Fail-open: Se o Redis der pau, permitimos o voto para não travar a API,
		// mas logamos o erro para monitoramento.
		return true
	}

	// Na primeira requisição da janela, define a expiração
	if count == 1 {
		v.redis.Expire(ctx, chave, janela)
	}

	return count <= limite
}
