# 🛡️ Vote API — O Produtor de Alta Performance e Seguro

> **Capacidade Alvo:** 5.000+ votos/segundo  
> **Stack:** Go 1.25 · Redis (Cache L1/L2) · RabbitMQ (Worker Pool) · Gin Framework

---

## 📐 Arquitetura e Escudo de Segurança

Este serviço atua como o ponto de entrada e o escudo principal do ecossistema de votação. Foi projetado para lidar com tráfego extremo enquanto protege o sistema contra duplicatas e ataques de inundação maliciosos (bots).

### O Pipeline de Ingestão de Votos:
1.  **CORS e Recuperação**: Restrição de domínio estrita e recuperação automática de falhas.
2.  **Rate Limiting por IP**: Limitador de janela fixa baseado em Redis (previne ataques de bots).
3.  **Validação JSON**: Mapeamento estrito do corpo da requisição com checagem de tipos.
4.  **Validação de Telefone**: Rejeita números malformados ou incompletos (< 8 dígitos).
5.  **Desduplicação (Camada Dupla)**:
    *   **L1 (Memória Local)**: Checagem instantânea na RAM local do servidor (< 1ms).
    *   **L2 (Redis)**: Checagem global distribuída via operação atômica `SETNX`.
6.  **Publicação Assíncrona**: O voto é enviado para um Pool de Workers interno e imediatamente confirmado para o usuário.

---

## 📁 Estrutura do Projeto

```text
api/
├── config/              # Configuração centralizada de produção (Estrita)
├── Rabiitmq/            #
│   ├── conexao/         # Gerenciamento de conexão TCP/TLS RabbitMQ
│   └── workerpool/      # Pool de produtores multithread (Buffer Interno)
├── router/              # Motor Gin, Rotas, Middlewares e Lógica de Segurança
├── validacao/           # O "Cérebro": Cache L1, Checagem Redis, Rate Limiting
├── models/              # Contratos de dados compartilhados
└── Dockerfile           # Build de produção multi-stage (Alpine Minimalista)
```

---

## ⚙️ Configuração (Variáveis de Ambiente)

Este serviço **falha imediatamente** se as variáveis críticas de produção estiverem ausentes.

| Variável | Obrigatoriedade | Descrição |
| :--- | :--- | :--- |
| `RABBITMQ_URL` | **Obrigatório** | `amqps://usuario:senha@host:porta/vhost` |
| `REDIS_URL` | **Obrigatório** | `redis://usuario:senha@host:porta` |
| `PORT` | Opcional | Porta da API (Padrão: `8080`) |
| `GIN_MODE` | Opcional | Use `release` para produção (Padrão: `release`) |
| `ALLOWED_ORIGIN` | Opcional | Restrição CORS (ex: `https://voto.com`). Padrão: `*` |

---

## 🚀 Endpoints da API

### 1. Enviar Voto
`POST /vote`

**Corpo da Requisição:**
```json
{
  "nome": "João Silva",
  "numero": 12345678,
  "emenda_votada": "Emenda 01"
}
```

**Respostas:**
*   `202 Accepted`: Voto recebido e enfileirado para processamento.
*   `400 Bad Request`: JSON inválido ou número de telefone muito curto.
*   `409 Conflict`: Voto duplicado detectado (já votou).
*   `429 Too Many Requests`: Limite de requisições por IP excedido (Proteção contra Bot).
*   `503 Service Unavailable`: Buffer interno cheio (Sobrecarga do Sistema).

### 2. Monitoramento de Saúde
`GET /health`

Fornece o status em tempo real das dependências críticas. Útil para Health Checks de Load Balancers (AWS/GCP/K8s).
*   **Verificações**: Conectividade com Redis, Saturação do Pool de Workers.

---

## ⚡ Recursos de Performance

### 1. Desduplicação em Camada Dupla
Para atingir 5 mil votos/s, não podemos consultar o Redis para cada duplicata. 
- **L1 (sync.Map)**: Armazena IDs vistos recentemente na memória local.
- **L2 (Redis)**: Garante a unicidade global entre todas as instâncias da API.

### 2. Pool de Workers Interno
Em vez de abrir um canal AMQP para cada requisição, a API usa um pool pré-alocado de **30 workers**. Cada worker tem seu próprio canal AMQP exclusivo, evitando problemas de concorrência e overhead de rede.

---

## 🛠️ Desenvolvimento e Deploy

### Rodar Localmente (para testes)
```bash
go mod tidy
SUBSTITUA_VARIAVEIS_AQUI go run main.go
```

### Build da Imagem de Produção
```bash
docker build -t vote-api .
```
