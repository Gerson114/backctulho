# 🗳️ vote-api — Documentação do Microserviço

> **Capacidade alvo:** 2.000 votos/segundo · **Stack:** Go · RabbitMQ · Redis

---

## 📐 Arquitetura Geral

```
Eleitor (HTTP)
    │  POST /vote
    ▼
┌──────────────────────────────────────────┐
│              vote-api                     │
│                                          │
│  1. Validação do Body (Gin)              │
│          ↓                               │
│  2. Anti-duplicata (Redis SET NX)        │
│          ↓                               │
│  3. WorkerPool.Publicar(voto)            │
│     ┌────┴────┐                          │
│     │ chan Go  │ ← fila interna (10.000) │
│     └─┬──┬──┬─┘                          │
│       │  │  │                            │
│     W0  W1 ... W19  ← 20 goroutines     │
│     ch0 ch1    ch19 ← canal próprio cada │
│       │  │  │                            │
│       └──┴──┘                            │
│          ↓                               │
│     RabbitMQ (fila "votos")              │
│                                          │
│  4. Responde 202 Accepted               │
└──────────────────────────────────────────┘
```

---

## 📁 Estrutura de Pastas

```
micro/api/
├── main.go                         ← ponto de entrada
├── models/
│   └── models.go                   ← struct Voto
├── validacao/
│   └── validacao.go                ← anti-duplicata (Redis SET NX)
└── Rabiitmq/
    ├── conexao/
    │   └── conexao.go              ← abre Connection com o broker
    ├── producer/
    │   └── producer.go             ← producer simples (canal único)
    └── workerpool/
        ├── workerpoll.go           ← struct WorkerPool, Novo(), Publicar()
        └── worker.go               ← goroutine com canal próprio
```

---

## 📦 Detalhamento de Cada Pacote

---

### 1. `models/models.go`

Define a estrutura de dados do voto que trafega pelo sistema.

```go
type Voto struct {
    ID           int64  `json:"id"`
    Nome         string `json:"nome"`
    Numero       int    `json:"numero"`
    EmendaVotada string `json:"emenda_votada"`
}
```

| Campo | Tipo | Descrição |
|---|---|---|
| `ID` | `int64` | Identificador único do voto |
| `Nome` | `string` | Nome do candidato ou referência |
| `Numero` | `int` | Número do candidato |
| `EmendaVotada` | `string` | Emenda escolhida pelo eleitor |

---

### 2. `Rabiitmq/conexao/conexao.go`

**Responsabilidade:** Abrir a conexão TCP com o broker RabbitMQ.

```
amqp.Dial(url) → *amqp.Connection
```

**Detalhes importantes:**
- Retorna **apenas** a `*amqp.Connection` (thread-safe)
- **Não** cria canais — os canais são criados pelo WorkerPool
- Se a conexão falhar, encerra o programa com `log.Fatalf`
- A URL aponta para o CloudAMQP (ambiente dev)

**Fluxo interno:**
```
ConexaoRabbitmq()
   │
   ├── amqp.Dial("amqps://...") → abre conexão TCP/TLS
   │       ↓
   ├── Se erro → log.Fatal (mata o programa)
   │       ↓
   └── return conn ✅
```

---

### 3. `validacao/validacao.go`

**Responsabilidade:** Garantir que um eleitor não vote duas vezes na mesma eleição.

**Mecanismo:** Redis `SET NX` (Set if Not eXists) — operação atômica, < 1ms.

```
Chave:  vote:{voterID}:election:{electionID}
Valor:  "1"
TTL:    7 dias (604.800 segundos)
```

**Fluxo interno:**
```
JaVotou(ctx, voterID, electionID)
   │
   ├── Redis SET NX chave "1" EX 604800
   │       ↓
   ├── Redis criou a chave? (primeira vez)
   │   ├── SIM → return nil (pode votar ✅)
   │   └── NÃO → return ErrVotoDuplicado ❌
   │       ↓
   └── Redis fora do ar? → return erro ⚠️
```

**Por que Redis e não banco de dados?**
- Redis: 100.000+ ops/segundo (responde em < 1ms)
- PostgreSQL: ~5.000 ops/segundo (responde em ~5ms)
- Para 2.000 req/s, Redis é 50x mais rápido

**Thread-safe:** `redis.Client` é seguro para uso concorrente — pode ser compartilhado entre goroutines.

---

### 4. `Rabiitmq/workerpool/workerpoll.go`

**Responsabilidade:** Gerenciar N goroutines que publicam votos no RabbitMQ em paralelo.

**Por que existe?**
`amqp.Channel` **não é thread-safe**. Se 2.000 requisições usarem o mesmo canal simultaneamente, o programa trava. O WorkerPool resolve isso dando **um canal exclusivo** para cada goroutine.

**Struct:**
```go
type WorkerPool struct {
    conn    *amqp.Connection  // thread-safe, compartilhada
    fila    chan models.Voto  // fila interna (buffer em memória)
    workers int               // quantidade de goroutines
}
```

**Funções:**

| Função | O que faz |
|---|---|
| `Novo(conn, 20, 10000)` | Cria o pool e dispara 20 goroutines |
| `iniciar()` | Loop que cria as goroutines com `go wp.worker(i)` |
| `Publicar(voto)` | API joga o voto na fila interna (~nanosegundos) |

**Back-pressure (Publicar):**
```go
select {
case wp.fila <- voto:  // enfileira e retorna
    return nil
default:               // fila cheia → rejeita imediatamente
    return fmt.Errorf("sistema sobrecarregado")
}
```
Isso impede que a API trave sob carga extrema — se a fila interna estiver cheia, responde 503 instantaneamente.

---

### 5. `Rabiitmq/workerpool/worker.go`

**Responsabilidade:** Cada goroutine abre seu canal RabbitMQ exclusivo e processa votos da fila interna.

**Fluxo de cada worker:**
```
worker(id)
   │
   ├── 1. Abre canal próprio: wp.conn.Channel()
   │      (canal exclusivo, sem concorrência)
   │
   ├── 2. Declara fila "votos" (idempotente)
   │      durable=true → sobrevive a restart
   │
   └── 3. Loop eterno:
          for voto := range wp.fila {
              json.Marshal(voto)
              canal.PublishWithContext(...)
          }
          ↑
          Bloqueia quando fila está vazia (zero CPU)
          Acorda quando um voto chega
```

**Detalhes da publicação:**
```go
amqp.Publishing{
    ContentType:  "application/json",
    DeliveryMode: amqp.Persistent,   // sobrevive ao restart do RabbitMQ
    Body:         corpo,             // voto serializado em JSON
}
```

**Se ocorrer erro:**
- Erro ao serializar → loga e descarta o voto (`continue`)
- Erro ao publicar → loga mas continua processando

---

### 6. `Rabiitmq/producer/producer.go`

**Responsabilidade:** Producer simples que usa um canal único compartilhado.

> ⚠️ **Este pacote existe para referência/testes.** Em produção, use o `workerpool` — o producer simples tem race condition com múltiplas goroutines.

| Diferença | `producer` | `workerpool` |
|---|---|---|
| Canais | 1 compartilhado | 1 por goroutine |
| Thread-safe | ❌ | ✅ |
| Throughput | ~500/s | ~5.000/s |
| Uso | Testes | Produção |

---

## 🔄 Fluxo Completo (Passo a Passo)

```
1. Eleitor envia POST /vote com JSON:
   { "id": 1, "nome": "João", "numero": 10, "emenda_votada": "E1" }

2. main.go recebeu a requisição
       ↓
3. validacao.JaVotou(voterID, electionID)
   └── Redis SET NX → chave existe?
       ├── SIM → 409 Conflict "já votou" ← para aqui
       └── NÃO → continua ↓

4. workerpool.Publicar(voto)
   └── wp.fila <- voto (coloca na fila interna)
       ├── Fila cheia → 503 Service Unavailable ← para aqui
       └── Enfileirou → continua ↓

5. Responde 202 Accepted ao eleitor ← API termina aqui

6. [Em background] Worker acorda
   └── pega voto da wp.fila
       └── json.Marshal(voto)
           └── canal.PublishWithContext(...)
               └── mensagem chega na fila "votos" do RabbitMQ ✅
```

---

## ⚡ Capacidade de Cada Componente

```
Componente            Capacidade       Meta (2.000/s)    Status
────────────────────  ───────────────  ────────────────  ──────
Redis SET NX          100.000+ ops/s   2.000 ops/s       ✅
WorkerPool (20)        ~5.000 pub/s    2.000 pub/s       ✅
Gin HTTP               30.000+ req/s   2.000 req/s       ✅
CloudAMQP (free)          ~1 msg/s     2.000 msg/s       ❌
RabbitMQ (cluster)    50.000+ msg/s    2.000 msg/s       ✅
```

> ⚠️ **O CloudAMQP gratuito é apenas para desenvolvimento.**
> Em produção, use um RabbitMQ Cluster próprio ou plano pago.

---

## 🚨 Pontos Críticos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Redis cai | Duplicatas passam | Redis Sentinel/Cluster + fallback PostgreSQL |
| RabbitMQ cai | Votos na fila interna se perdem | `amqp.Persistent` + DLQ + retry |
| WorkerPool cheio | 503 para o eleitor | Aumentar `buffer` (10.000 → 50.000) |
| Canal do worker morre | Worker para de publicar | Recriar canal automaticamente |
| Pico acima de 5.000/s | Fila interna cresce | Mais workers (20 → 50) |

---

## 🏃 Como Executar

```bash
cd micro/api

# Instalar dependências
go mod tidy

# Rodar
go run .
# Output: "RabbitMQ: conectado com sucesso"
#         "worker 0 pronto para receber votos"
#         ...
#         "worker 19 pronto para receber votos"
```

---

## 📚 Dependências

| Pacote | Versão | Uso |
|---|---|---|
| `github.com/rabbitmq/amqp091-go` | v1.10.0 | Cliente AMQP para RabbitMQ |
| `github.com/redis/go-redis/v9` | v9.18.0 | Cliente Redis (anti-duplicata) |
| `github.com/gin-gonic/gin` | v1.12.0 | Framework HTTP |
| `github.com/google/uuid` | v1.6.0 | Geração de UUIDs |
