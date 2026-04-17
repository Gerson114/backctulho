package validacao

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/bits-and-blooms/bloom/v3"
	"sync"
)

var ErrVotoDuplicado = errors.New("eleitor já votou em uma emenda e só é permitido um voto por pessoa")

type Validador struct {
	redis     *redis.Client
	cacheL1   *lru.Cache[string, struct{}] // Cache em memória (LRU)
	bloom     *bloom.BloomFilter           // Escudo de Memória (Bloom Filter)
	bloomMu   sync.RWMutex                 // Proteção para concorrência
}

func Novo(rdb *redis.Client) *Validador {
	// 1. Inicializa cache L1 com limite de 50.000 (Itens recentes)
	cache, _ := lru.New[string, struct{}](50_000)
	
	// 2. Inicializa Bloom Filter para 1 Milhão de itens (apenas 1.2MB de RAM)
	// Precisão de 0.1% (1 erro a cada 1000 pode disparar o Redis desnecessariamente)
	filter := bloom.NewWithEstimates(1_000_000, 0.001)

	return &Validador{
		redis:   rdb,
		cacheL1: cache,
		bloom:   filter,
	}
}

func (v *Validador) JaVotou(ctx context.Context, voterID string, electionID string) error {
	chave := fmt.Sprintf("vote_global:%s", voterID)

	// --- 1. ESCUDO DE MEMÓRIA (Bloom Filter) | Latência: ~0ms ---
	// Se o Bloom Filter diz que NÃO está lá, temos 100% de certeza.
    v.bloomMu.RLock()
    jaVotouBloom := v.bloom.TestString(chave)
    v.bloomMu.RUnlock()

	if !jaVotouBloom {
		// É um voto novo! Adicionamos ao Bloom e liberamos instantaneamente.
        v.bloomMu.Lock()
        v.bloom.AddString(chave)
        v.bloomMu.Unlock()
		
		// Agendamos a gravação no Redis em background para manter a "Verdade Absoluta" 
		// mas sem segurar o usuário na linha.
		go func() {
			ctxBg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = v.redis.SetNX(ctxBg, chave, "1", 7*24*time.Hour).Result()
		}()
		
		return nil
	}

	// --- 2. VERIFICAÇÃO DE SEGURANÇA (Redis) | Só ocorre em suspeitas de duplicidade ---
	// Se o Bloom Filter disse "Talvez", ou se foi reiniciado, consultamos o Redis.
	criou, err := v.redis.SetNX(ctx, chave, "1", 7*24*time.Hour).Result()
	if err != nil {
		// Se o Redis falhar, deixamos passar (fail-open) para não travar a votação,
		// o Bloom já nos deu uma boa segurança acima.
		return nil 
	}

	if !criou {
		return ErrVotoDuplicado
	}

	return nil
}

// CarregarFiltro lê todos os votos existentes no Redis e carrega na memória.
// Deve ser chamado apenas no início da API.
func (v *Validador) CarregarFiltro(ctx context.Context) error {
	var cursor uint64
	var count int
	log.Printf("[SISTEMA] 🧊 Carregando votos do Redis para o Bloom Filter (Warm-up)...")

	for {
		keys, nextCursor, err := v.redis.Scan(ctx, cursor, "vote_global:*", 1000).Result()
		if err != nil {
			return fmt.Errorf("erro no scan do redis: %w", err)
		}

		v.bloomMu.Lock()
		for _, k := range keys {
			v.bloom.AddString(k)
			count++
		}
		v.bloomMu.Unlock()

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	log.Printf("[SISTEMA] ✨ Warm-up concluído! %d votos carregados na memória.", count)
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

// GetVotosEmenda retorna a quantidade de votos de uma emenda específica direto do Redis.
func (v *Validador) GetVotosEmenda(ctx context.Context, emendaID string) (int64, error) {
	val, err := v.redis.HGet(ctx, "votos:emendas", emendaID).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// GetTotalVotos retorna o contador global de votos.
func (v *Validador) GetTotalVotos(ctx context.Context) (int64, error) {
	val, err := v.redis.Get(ctx, "votos:total").Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// GetVotosMapa retorna todos os votos de todas as emendas em um mapa ID -> Total.
func (v *Validador) GetVotosMapa(ctx context.Context) (map[string]int64, error) {
	res, err := v.redis.HGetAll(ctx, "votos:emendas").Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return make(map[string]int64), nil
		}
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

// Subscribe retorna um PubSub do Redis para ouvir canais em tempo real.
func (v *Validador) Subscribe(ctx context.Context, canal string) *redis.PubSub {
	return v.redis.Subscribe(ctx, canal)
}
