package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func Conectar(url string) *Client {
	opts, err := redis.ParseURL(url)
	if err != nil {
		log.Fatalf("❌ Redis: falha ao parsear URL: %v", err)
	}

	rdb := redis.NewClient(opts)

	// Teste de conexão
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("❌ Redis: falha ao conectar: %v", err)
	}

	return &Client{rdb: rdb}
}

func (c *Client) Fechar() {
	_ = c.rdb.Close()
}

// IncrementarVotos realiza o incremento atômico no Redis para a emenda e para o total.
func (c *Client) IncrementarVotos(ctx context.Context, emendaID string) error {
	pipe := c.rdb.Pipeline()
	pipe.Incr(ctx, "votos:total")
	if emendaID != "" {
		pipe.HIncrBy(ctx, "votos:emendas", emendaID, 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// IncrementarLoteVotos incrementa o total e uma emenda específica com um valor determinado (Lote).
func (c *Client) IncrementarLoteVotos(ctx context.Context, emendaID string, count int64) error {
	pipe := c.rdb.Pipeline()
	pipe.IncrBy(ctx, "votos:total", count)
	if emendaID != "" {
		pipe.HIncrBy(ctx, "votos:emendas", emendaID, count)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// PublicarVotos envia um lote de atualizações para o canal Pub/Sub do Redis.
func (c *Client) PublicarVotos(ctx context.Context, canal string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.rdb.Publish(ctx, canal, payload).Err()
}

// GetTodosVotosEmendas retorna o mapa completo de votos por emenda do Redis.
func (c *Client) GetTodosVotosEmendas(ctx context.Context) (map[string]int64, error) {
	res, err := c.rdb.HGetAll(ctx, "votos:emendas").Result()
	if err != nil {
		return nil, err
	}

	mapa := make(map[string]int64)
	for k, v := range res {
		var val int64
		_, _ = fmt.Sscanf(v, "%d", &val)
		mapa[k] = val
	}
	return mapa, nil
}

// GetVotosEmenda retorna a contagem de uma única emenda.
func (c *Client) GetVotosEmenda(ctx context.Context, emendaID string) (int64, error) {
	val, err := c.rdb.HGet(ctx, "votos:emendas", emendaID).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// Subscribe assina um canal do Redis e retorna o canal de mensagens.
func (c *Client) Subscribe(ctx context.Context, canal string) <-chan *redis.Message {
	pubsub := c.rdb.Subscribe(ctx, canal)
	return pubsub.Channel()
}
