# Consumidor RabbitMQ de Alta Performance – Documentação Completa

## Sumário
1. Visão geral do sistema
2. Arquitetura detalhada
3. Fluxo de dados (do RabbitMQ ao PostgreSQL)
4. Configurações (`.env`)
5. Estrutura de código (pacotes e responsabilidades)
6. Como executar a aplicação
7. Como habilitar a persistência no banco
8. Estratégias de performance e tuning
9. Teste de carga (`automacao/automacao.go`)
10. Solução de problemas
11. Melhorias futuras
12. Perguntas frequentes (FAQ)

---

## 1. Visão geral do sistema
O objetivo deste projeto é consumir mensagens de voto enviadas a uma fila RabbitMQ a uma taxa superior a **1.000 votos/segundo**, processá‑las em lote e gravá‑las de forma eficiente no PostgreSQL. O design prioriza:
- **Baixa latência** – prefetch agressivo e workers concorrentes.
- **Alto throughput** – inserções em lote (`CreateInBatches`).
- **Escalabilidade** – parâmetros configuráveis via variáveis de ambiente.
- **Resiliência** – cancelamento de contexto e tratamento de erros.

---

## 2. Arquitetura detalhada
```
+-------------------+      +-------------------+      +-------------------+      +-------------------+
| RabbitMQ Queue    | ---> | Consumer (Go)     | ---> | Worker Pool       | ---> | Handler (Postgres) |
| (votos)           |      | - QoS (prefetch)  |      | - Batching        |      | - GORM bulk insert |
+-------------------+      +-------------------+      +-------------------+      +-------------------+
```
### Componentes
| Componente | Responsabilidade |
|------------|------------------|
| **Config (`config/config.go`)** | Carrega variáveis de ambiente (`RABBITMQ_URL`, `NUM_WORKERS`, `BATCH_SIZE`, `BATCH_TIMEOUT`, `PREFETCH_COUNT`, `POSTGRES_DSN`). |
| **Consumer (`rabbitmq/consumer/consumer.go`)** | Conecta ao RabbitMQ, define QoS (`prefetchCount`) e entrega mensagens ao canal `messages`. |
| **Worker Pool (`rabbitmq/workers/workers.go`)** | Cria `NUM_WORKERS` goroutines, agrupa mensagens em lotes de `BATCH_SIZE` e dispara o `handler` quando o lote está completo ou o `BATCH_TIMEOUT` expira. |
| **Handler (`handlers/votos.go`)** | Converte `amqp.Delivery` (JSON) em `models.Voto` e persiste usando `db.CreateInBatches`. |
| **DB Layer (`db/database.go`)** | Inicializa GORM, configura pool de conexões e fornece o objeto `*gorm.DB`. |
| **Main (`main.go`)** | Orquestra a inicialização (config → DB → Consumer → WorkerPool). |

---

## 3. Fluxo de dados (RabbitMQ → PostgreSQL)
1. **Início** – `main.go` carrega a configuração e cria o contexto cancelável.
2. **Conexão RabbitMQ** – `consumer.StartConsumer` abre um canal, aplica `ch.Qos(prefetchCount, 0, false)` e começa a consumir.
3. **Distribuição** – Cada mensagem (`amqp.Delivery`) é enviada ao canal `messages`.
4. **Worker Pool** – Cada worker recebe mensagens, reconhece (`msg.Ack(false)`) imediatamente, acumula em `batchCount`.
5. **Flush** – Quando `batchCount >= BATCH_SIZE` **ou** o ticker (`BATCH_TIMEOUT`) dispara, o worker chama o `handler` passando o slice de `amqp.Delivery`.
6. **Handler** – Deserializa JSON → `models.Voto` → `db.CreateInBatches(votos, cfg.BatchSize)` (uma única operação `INSERT`).
7. **Commit** – O batch é inserido dentro de uma transação (commit único), reduzindo overhead de I/O.
8. **Log** – O logger de alta performance registra o tamanho do lote processado.

---

## 4. Configurações (`.env`)
```dotenv
# Conexão RabbitMQ
RABBITMQ_URL=amqp://guest:guest@127.0.0.1:5672/
QUEUE_NAME=minha_fila

# Worker pool
NUM_WORKERS=40          # número de goroutines workers
BATCH_SIZE=700          # mensagens por lote
BATCH_TIMEOUT=500ms     # tempo máximo antes de flush de lote parcial
PREFETCH_COUNT=20000    # mensagens pré‑buscadas no canal RabbitMQ

# Banco de dados PostgreSQL (opcional – habilite no handler)
POSTGRES_DSN=postgres://postgres:root@localhost:5432/votos?sslmode=disable
```
> **Dicas:**
> - `NUM_WORKERS` deve ser ≤ número de núcleos físicos * 2 (para evitar oversubscription). 
> - `PREFETCH_COUNT` controla quantas mensagens podem ficar “não acked”. Valores muito altos podem consumir muita memória no RabbitMQ.
> - `BATCH_TIMEOUT` **não pode ser zero**; caso esteja ausente o código usa `1s` como fallback.

---

## 5. Estrutura de código (pacotes)
```
├─ config/            # carregamento de .env
├─ db/                # camada GORM, pool de conexões
├─ handlers/          # lógica de persistência (votos.go)
├─ rabbitmq/
│   ├─ consumer/     # StartConsumer, QoS
│   └─ workers/      # StartWorkerPool, ticker, batch
├─ models/            # structs de domínio (Voto)
├─ main.go            # ponto de entrada
└─ go.mod
```
Cada pacote tem **uma única responsabilidade** (princípio SRP), facilitando testes unitários e manutenção.

---

## 6. Como executar a aplicação
```powershell
# 1. Clone o repositório (já está na sua máquina)
cd C:\Users\Gerson Firmino\Desktop\r\backctulho\dados

# 2. Crie/edite o .env conforme seu ambiente
notepad .env   # ou use seu editor preferido

# 3. Instale dependências (go modules)
go mod tidy

# 4. Rode a aplicação
go run main.go
```
Saída esperada:
```
[App] Inicializando conexão com RabbitMQ...
[SISTEMA] Modo Turbo: Prefetch 20000 | Workers 40 | Lote 700
[15:04:05] 🚀 LOTE PROCESSADO: 700 mensagens consumidas.
```
Pressione **Ctrl+C** para encerrar; o contexto será cancelado e todos os workers finalizarão graciosamente.

---

## 7. Como habilitar a persistência no banco
1. **Descomente a inicialização do DB** em `app/app.go` (linhas que criam `db.NewDatabase(cfg.DBDSN)`).
2. **Altere o handler** (`handlers/votos.go`) para usar a função `ProcessVoteBatch` que faz `db.CreateInBatches`. O código já está preparado; basta remover os comentários que ignoram o DB.
3. **Crie o banco** (se ainda não existir):
```bash
createdb votos -U postgres
psql -U postgres -d votos -c "CREATE EXTENSION IF NOT EXISTS "uuid-ossp";"
```
4. **Execute migração** (opcional) – crie a tabela `votos` com:
```sql
CREATE TABLE votos (
    id          BIGSERIAL PRIMARY KEY,
    nome        TEXT NOT NULL,
    numero      INT NOT NULL,
    emenda_votada TEXT,
    votado_em   TIMESTAMP WITH TIME ZONE NOT NULL,
    ip_origem   TEXT
);

CREATE INDEX idx_votos_votado_em ON votos (votado_em);
CREATE INDEX idx_votos_numero ON votos (numero);
```
5. **Re‑inicie a aplicação** – agora os lotes serão gravados no PostgreSQL.

---

## 8. Estratégias de performance e tuning
| Parâmetro | Recomendações | Onde mudar |
|-----------|---------------|------------|
| `NUM_WORKERS` | 20‑80 (dependendo dos núcleos). | `.env` → `NUM_WORKERS` |
| `PREFETCH_COUNT` | 10 000‑50 000. Monitorar memória do RabbitMQ. | `.env` → `PREFETCH_COUNT` |
| `BATCH_SIZE` | 500‑2000. Maior batch = menos round‑trips ao DB. | `.env` → `BATCH_SIZE` |
| `BATCH_TIMEOUT` | 100‑1000 ms. Evita lotes pequenos quando o tráfego é baixo. | `.env` → `BATCH_TIMEOUT` |
| **Pool de conexões** | `SetMaxOpenConns = NUM_WORKERS * 2` ; `SetMaxIdleConns = NUM_WORKERS`. | `db/database.go` |
| **WAL** | `synchronous_commit = off` (se tolerar perda de alguns segundos). `wal_level = minimal`. | `postgresql.conf` |
| **Índices** | Mantenha apenas os necessários; crie após carga massiva se precisar. | SQL de migração |
| **Hardware** | SSD recomendado; 8 GB RAM mínimo para 1 k votos/s. | – |

### Benchmark rápido (local)
```bash
# Simular 20 000 votos/s
go run automacao/automacao.go -workers 2000
```
Observe a taxa de inserção no PostgreSQL (`SELECT count(*) FROM votos;`) e a latência do consumidor nos logs.

---

## 9. Teste de carga (`automacao/automacao.go`)
O script já está configurado para gerar requisições HTTP ao endpoint `/vote`. Ajuste a variável `workers` dentro do arquivo para mudar a carga. Exemplo de saída:
```
👥 INICIANDO SIMULAÇÃO HUMANA
🎯 Alvo: http://localhost:8080/vote | Workers: 2000
📊 Total: 10000 | Ritmo: 1031 votos/s | Status: OK (200)
```
Use esse número como referência para validar se o consumidor está acompanhando a geração de mensagens.

---

## 10. Solução de problemas
| Sintoma | Causa provável | Correção |
|---------|----------------|----------|
| **Panic: intervalo não‑positivo para NewTicker** | `BATCH_TIMEOUT` ausente ou `0`. | Defina um valor válido (`500ms`). O código já tem fallback de `1s`. |
| **Timeout de 10s com workers lentos** | Handler está fazendo I/O pesado (ex.: escrita síncrona no DB). | Use inserções em lote, aumente `BATCH_TIMEOUT` ou diminua `BATCH_SIZE`. |
| **Erro de conexão ao RabbitMQ** | URL ou credenciais incorretas. | Verifique `RABBITMQ_URL` e permissões no vhost. |
| **Erro de conexão ao Postgres** | DSN errado, banco não existe ou usuário sem permissão. | Crie o banco `votos` e ajuste `POSTGRES_DSN`. |
| **Too many connections** | `max_connections` do PostgreSQL atingido. | Aumente `max_connections` ou reduza `NUM_WORKERS`. |
| **Alto uso de CPU** | Número de workers maior que a capacidade da CPU. | Reduza `NUM_WORKERS` ou aumente a máquina. |

---

## 11. Melhorias futuras
- **Escalonamento automático** – monitorar a profundidade da fila e ajustar `NUM_WORKERS`/`PREFETCH_COUNT` dinamicamente.
- **Back‑pressure** – pausar o consumo quando o DB estiver saturado.
- **Métricas Prometheus** – expor contadores de lotes processados, latência, tamanho da fila.
- **Docker/Kubernetes** – containerizar a aplicação e usar Helm para deploy.
- **Replica read‑only** – separar carga de leitura (consultas) da escrita (inserções). 
- **Uso de `COPY`** – para ingestão ainda mais rápida em cenários de carga extrema.

---

## 12. Perguntas frequentes (FAQ)
**Q:** *Preciso usar o PostgreSQL?*  
**A:** Não. O handler pode ficar em modo “log‑only”. Quando quiser persistir, habilite o DB conforme a seção 7.

**Q:** *Posso mudar as configurações sem reiniciar?*  
**A:** Não. As variáveis são lidas apenas na inicialização.

**Q:** *O que acontece se o consumidor perder a conexão com o RabbitMQ?*  
**A:** O código atual tenta reconectar na próxima tentativa de `StartConsumer`. Você pode melhorar adicionando lógica de reconexão exponencial.

**Q:** *Como faço backup dos dados?*  
**A:** Use `pg_dump` ou configure replicação streaming.

---

*Documentação gerada em 2026‑04‑08*
