# STAGE 1: Builder
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compila o binário do consumer
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o consumer main.go

# STAGE 2: Runner
FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/consumer .

# O consumer não precisa de portas expostas (ele que inicia as conexões)

ENTRYPOINT ["./consumer"]
