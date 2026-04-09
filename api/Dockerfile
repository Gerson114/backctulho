# STAGE 1: Builder
# Usamos a imagem oficial do Go para compilar o binário em um ambiente isolado
FROM golang:1.25-alpine AS builder

# Instalamos dependências necessárias para compilar pacotes C (se necessário)
RUN apk add --no-cache git gcc musl-dev

# Define o diretório de trabalho dentro do container
WORKDIR /app

# Copia os arquivos de definição de dependência primeiro (otimização de cache do Docker)
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante do código-fonte
COPY . .

# Compila o binário de produção
# CGO_ENABLED=0 garante um binário estático que roda em qualquer Linux
# -ldflags="-w -s" remove símbolos de debug para diminuir o tamanho do arquivo
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server main.go

# STAGE 2: Runner
# Imagem final mínima para segurança e performance
FROM alpine:3.18

# Adicionamos certificados de CA para permitir conexões HTTPS (ex: Redis Cloud, AMQPS)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copia o binário do builder para a imagem final
COPY --from=builder /app/server .

# Expõe a porta padrão do serviço
EXPOSE 8080

# Comando de inicialização
# Em produção, as variáveis de ambiente devem ser passadas via infra (AWS/Fly/Kubernetes)
ENTRYPOINT ["./server"]
