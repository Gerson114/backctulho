package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"meu-projeto/Rabiitmq/workerpool"
	"meu-projeto/models"
	"meu-projeto/validacao"
)

// Iniciar cria o engine Gin com as rotas registradas.
func Iniciar(wp *workerpool.WorkerPool, val *validacao.Validador) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// Middleware CORS para permitir requisições do frontend
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})



	r.POST("/vote", func(c *gin.Context) {
		var voto models.Voto

		// 1. Valida o body JSON
		if err := c.ShouldBindJSON(&voto); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":   "erro",
				"mensagem": "body inválido: " + err.Error(),
			})
			return
		}

		// 2. Preenche campos de rastreabilidade
		voto.VotadoEm = time.Now().UTC()
		voto.IPOrigem = c.ClientIP()

		// 3. Verifica duplicata via Redis
		// A chave de segurança agora é o Número do Telefone passado no frontend
		// Se ela enviar em branco a emenda, usamos fallback "emenda-default" para evitar bugs.
		voterID := fmt.Sprintf("%d", voto.Numero)
		electionID := voto.EmendaVotada
		if electionID == "" {
			electionID = "emenda-default"
		}
		
		ctxRedis, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
		defer cancel()

		if err := val.JaVotou(ctxRedis, voterID, electionID); err != nil {
			if errors.Is(err, validacao.ErrVotoDuplicado) {
				c.JSON(http.StatusConflict, gin.H{
					"status":   "duplicado",
					"mensagem": "eleitor já votou nesta eleição",
				})
				return
			}
			// Redis indisponível — loga mas deixa o voto passar
			_ = err
		}

		// 4. Publica na fila interna → workers → RabbitMQ
		if err := wp.Publicar(voto); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "erro",
				"mensagem": "sistema sobrecarregado, tente novamente",
			})
			return
		}

		// 5. Responde imediatamente (assíncrono)
		c.JSON(http.StatusAccepted, gin.H{
			"status":   "aceito",
			"mensagem": "voto recebido com sucesso",
		})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}
