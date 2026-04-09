package connect

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ConnectRabbitMQ estabelece a conexão TCP principal com o RabbitMQ com logs de diagnóstico.
func ConnectRabbitMQ(rawUrl string) (*amqp.Connection, error) {
	cleanURL := strings.TrimSpace(rawUrl)

	if cleanURL == "EMPTY" || cleanURL == "" {
		return nil, fmt.Errorf("URL do RabbitMQ está vazia ou não foi carregada corretamente do .env")
	}

	// Mascarar senha para o log
	masked := maskURL(cleanURL)
	log.Printf("[AMQP] Tentando conectar em: %s", masked)

	conn, err := amqp.Dial(cleanURL)
	if err != nil {
		return nil, fmt.Errorf("erro no Dial: %w", err)
	}

	log.Println("[AMQP] ✅ Conexão TCP estabelecida com sucesso.")
	return conn, nil
}

// OpenChannel abre um novo canal AMQP a partir de uma conexão existente.
func OpenChannel(conn *amqp.Connection) (*amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir canal AMQP: %w", err)
	}
	return ch, nil
}

// maskURL oculta a senha da URL para logs seguros
func maskURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "URL_INVALIDA"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "********")
	}
	return u.String()
}
