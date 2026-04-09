# 🚀 Vote Data Consumer — Motor de Ingestão de Alta Performance

> **Alvo de Throughput:** 5.000+ registros/segundo  
> **Stack:** Go 1.25 · RabbitMQ (Múltiplos Canais) · PostgreSQL · GORM (Inserção em Lote)

---

## 🏗️ Arquitetura: O Pipeline de 2 Estágios

Para atingir o throughput máximo e contornar os gargalos de canal único do RabbitMQ, este serviço implementa um pipeline de processamento desacoplado.

### Estágio 1: Os Coletores (Ingestão de Rede)
- **Paralelismo**: Múltiplos canais AMQP independentes (Padrão: 3).
- **Ajuste de QoS**: `PREFETCH_COUNT` agressivo (10.000+) garante que o buffer local do consumidor nunca fique vazio.
- **Objetivo**: Extrair dados da rede o mais rápido que a largura de banda permitir.

### Estágio 2: Os Processadores (Persistência de Dados)
- **Agrupamento (Batching)**: Coleta mensagens em grandes fatias (Padrão: 1.000 msgs).
- **Concorrência**: Pool de workers de processamento (Padrão: 60) aguardando lotes cheios.
- **Persistência**: Usa o `CreateInBatches` do GORM para realizar operações massivas de `INSERT` em uma única ida ao banco de dados.

---

## ⚙️ Configuração (Parâmetros de Tuning)

Este serviço utiliza validação estrita de ambiente para estabilidade em produção.

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `RABBITMQ_URL` | **Obrigatório** | String de conexão para o CloudAMQP ou cluster local. |
| `POSTGRES_DSN` | **Obrigatório** | DSN do PostgreSQL para persistência dos votos. |
| `NUM_COLLECTORS` | `3` | Número de canais AMQP independentes a serem abertos. |
| `NUM_PROCESSORS` | `60` | Goroutines paralelas para gerenciar a inserção no banco. |
| `BATCH_SIZE` | `1000` | Número de votos por operação de inserção em lote. |
| `PREFETCH_COUNT` | `10000` | Mensagens permitidas em trânsito por coletor. |

---

## 📁 Estrutura do Projeto

```text
dados/
├── app/                 # Orquestração (gerenciamento de ciclo de vida)
├── config/              # Validador de ambiente estrito
├── db/                  # Ajuste de pool do banco (MaxOpenConns, MaxIdleConns)
├── handlers/            # Processamento de lote e mapeamento GORM
├── models/              # Contratos de dados compartilhados
├── rabbitmq/            #
│   ├── connect/         # Lógica de conexão e TLS
│   ├── consumer/        # Lógica de Coleta (Estágio 1)
│   └── workers/         # Lógica de Processamento (Estágio 2)
└── Dockerfile           # Build de produção minimalista
```

---

## ⚡ Tuning de Performance (PostgreSQL)

O pool de conexões é ajustado automaticamente para corresponder ao número de processadores:
- **MaxOpenConns**: `100` (Permite commits de lote em alta frequência).
- **MaxIdleConns**: `50` (Mantém conexões prontas para o pool de workers).
- **ConnMaxLifetime**: `1 Hora`.

---

## 🛠️ Deploy e Manutenção

### Graceful Shutdown (Encerramento Suave)
O consumidor implementa uma estratégia de saída limpa. Quando recebe um sinal de término (`SIGTERM`):
1. Os Coletores param de receber novas mensagens.
2. O buffer interno é esvaziado e os lotes atuais são gravados no banco.
3. As conexões com o Banco e RabbitMQ são fechadas com segurança.

### Monitoramento
Verifique os logs do sistema para:
- `🚀 LOTE PROCESSADO`: Confirmação visual do tamanho e velocidade do lote.
- `⚠️ Reconectando`: Indicador de instabilidade na rede do RabbitMQ.

---

## 🏃 Como Rodar

### Produção (Recomendado)
Use o `docker-compose.yml` da raiz para lançar junto com a API.

### Desenvolvimento
```bash
go mod tidy
SUBSTITUA_VARIAVEIS_AQUI go run main.go
```
