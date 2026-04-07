package validacao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrVotoDuplicado = errors.New("eleitor já votou em uma emenda e só é permitido um voto por pessoa")

type Validador struct {
	redis *redis.Client
}

func Novo(rdb *redis.Client) *Validador {
	return &Validador{redis: rdb}
}

func (v *Validador) JaVotou(ctx context.Context, voterID string, electionID string) error {
	// A chave agora ignora a emenda ("electionID"), tornando o bloqueio Global
	chave := fmt.Sprintf("vote_global:%s", voterID)

	criou, err := v.redis.SetNX(ctx, chave, "1", 7*24*time.Hour).Result()
	if err != nil {
		return fmt.Errorf("redis indisponível: %w", err)
	}
	if !criou {
		return ErrVotoDuplicado
	}
	return nil
}

// PermitirRateLimit cria um bloqueio simples de janela fixa (Fixed Window Counter) via Redis.
// Retorna false se o IP enviou mais requisições que o "limite" dentro do tempo "janela".
func (v *Validador) PermitirRateLimit(ctx context.Context, ip string, limite int64, janela time.Duration) bool {
	chave := fmt.Sprintf("ratelimit:ip:%s", ip)

	count, err := v.redis.Incr(ctx, chave).Result()
	if err != nil {
		// Fail-open: Se estourar 50ms ou o Redis der pau de latência, deixa conectar para a API não cair.
		return true
	}

	if count == 1 {
		v.redis.Expire(ctx, chave, janela)
	}

	return count <= limite
}
