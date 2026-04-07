package validacao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrVotoDuplicado = errors.New("eleitor já votou nesta eleição")

type Validador struct {
	redis *redis.Client
}

func Novo(rdb *redis.Client) *Validador {
	return &Validador{redis: rdb}
}

func (v *Validador) JaVotou(ctx context.Context, voterID string, electionID string) error {
	chave := fmt.Sprintf("vote:%s:election:%s", voterID, electionID)

	criou, err := v.redis.SetNX(ctx, chave, "1", 7*24*time.Hour).Result()
	if err != nil {
		return fmt.Errorf("redis indisponível: %w", err)
	}
	if !criou {
		return ErrVotoDuplicado
	}
	return nil
}
