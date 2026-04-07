package models

import "time"

// Voto representa o payload publicado no RabbitMQ.
type Voto struct {
	ID           int64     `json:"id"`
	Nome         string    `json:"nome"`
	Numero       int       `json:"numero"`
	EmendaVotada string    `json:"emenda_votada"`
	VotadoEm    time.Time `json:"votado_em"`   // timestamp do momento do voto
	IPOrigem    string    `json:"ip_origem"`   // rastreabilidade
}
