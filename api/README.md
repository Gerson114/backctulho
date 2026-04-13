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

## ☁️ Arquitetura de Deploy Nativa AWS (Produção)

Para garantir que a API suporte **5.000+ votos/segundo** sem engasgos, esta é a topologia recomendada focada 100% no ecossistema AWS:

### 1. Computação (A API Go)
Recomendamos o uso do **Amazon ECS com AWS Fargate** ou de múltiplas instâncias no **Amazon EC2**.
- **Amazon EC2**: Instâncias da família *Compute Optimized* ou *Burstable* de nova geração (ex: `c6g.large` com processadores Graviton, ou `t3.medium`).
- **Amazon ECS (Fargate)**: Se preferir focar apenas no código, empacote o Dockerfile e rode tasks Serverless. Alocação ideal: 2 vCPUs e 4GB RAM por task.
- **Load Balancer**: A aplicação não deve receber tráfego direto. Coloque as instâncias/tasks atrás de um **Application Load Balancer (ALB)**. O ALB vai gerenciar os certificados SSL (gratuitos via AWS Certificate Manager) e vai entregar apenas tráfego HTTP limpo na porta `8080` para o backend em Go, reduzindo o custo computacional com criptografia.
- **Auto Scaling**: Configure um Auto Scaling Group baseado no consumo de CPU (alvo: 60%). Como o Go inicia em poucos milissegundos, novas máquinas subirão instantaneamente sob alto estresse.

### 2. Infraestrutura de Apoio (As Travas)
Para sustentar 5.000 RPS, o disco e a CPU das máquinas EC2 de aplicação não podem concorrer com o Redis ou filas. Tudo deve ser fatiado:
- **Cache Mestre (Redis)**: Use o **Amazon ElastiCache para Redis**. Crie o cluster na mesma sub-rede privada (VPC) e mesma Região (ex: `sa-east-1` - São Paulo) que a API. A latência entre a EC2 e o ElastiCache precisa ser consistentemente inferior a 1 milissegundo para o rate limit e a checagem global (`SETNX`) performarem.
- **Mensageria (RabbitMQ)**: Utilize o **Amazon MQ for RabbitMQ**. Ele cuidará ativamente de espelhar as mensagens e garantir que não travem. Em picos absurdos, se o motor de banco de dados atrasar, o Amazon MQ vai segurar com segurança no disco SSD dele as mensagens até desafogar.
- **Banco de Dados**: Com a API entregando ao Amazon MQ, a responsabilidade cai na máquina 'consumer' (`dados`), que deverá inserir os fardos (batching) em um **Amazon Aurora PostgreSQL**. O Aurora lidará tranquilamente com as transações em massa sob alta frequência de discos virtuais NVMe.

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
