package router

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"meu-projeto/Rabiitmq/workerpool"
	"meu-projeto/config"
	"meu-projeto/models"
	"meu-projeto/validacao"
)

// Iniciar cria o engine Gin com as rotas registradas e proteções de segurança.
func Iniciar(wp *workerpool.WorkerPool, val *validacao.Validador, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())

	// 🛡️ SEGURANÇA: Configuração de Proxies Confiáveis
	// Por padrão, não confiamos em nenhum proxy para evitar IP Spoofing via cabeçalhos X-Forwarded-For.
	// Se usar Cloudflare/Nginx/ALB, adicione os IPs dos seus balanceadores aqui.
	r.SetTrustedProxies(nil)

	// Middleware CORS: Suporta múltiplos domínios (localhost e IP da rede)
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOrigins := strings.Split(cfg.AllowedOrigin, ",")

		// Se a origem da requisição estiver na lista permitida, nós a ecoamos no header
		for _, o := range allowedOrigins {
			if strings.TrimSpace(o) == origin {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

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
		// 🛡️ SEGURANÇA 1: RATE LIMIT (DESATIVADO TEMPORARIAMENTE PARA TESTE DE CARGA)
		/*
			ip := c.ClientIP()
			if !val.PermitirRateLimit(c.Request.Context(), ip, 5, 10*time.Second) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"status":   "erro",
					"mensagem": "muitas requisições, por favor aguarde alguns segundos",
				})
				c.Abort()
				return
			}
		*/
		ip := c.ClientIP()

		var voto models.Voto

		// 🛡️ SEGURANÇA 2: VALIDAÇÃO DE CORPO
		if err := c.ShouldBindJSON(&voto); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":   "erro",
				"mensagem": "formato de dados inválido ou corpo vazio",
			})
			return
		}

		// 🛡️ SEGURANÇA 3: VALIDAÇÃO DE DADOS (Telefone Mínimo)
		// Impede números falsos ou curtos demais (mínimo 8 dígitos)
		if voto.Numero < 10_000_000 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":   "erro",
				"mensagem": "número de telefone inválido ou incompleto",
			})
			return
		}

		// Rastreabilidade
		voto.VotadoEm = time.Now().UTC()
		voto.IPOrigem = ip

		// 🛡️ SEGURANÇA 4: VERIFICAÇÃO DE DUPLICIDADE GLOBAL
		voterID := fmt.Sprintf("%d", voto.Numero)
		electionID := voto.EmendaVotada
		if electionID == "" {
			electionID = "emenda-default"
		}

		// Timeout aumentado para 5s para aguentar latência de rede em produção
		ctxRedis, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		if err := val.JaVotou(ctxRedis, voterID, electionID); err != nil {
			if errors.Is(err, validacao.ErrVotoDuplicado) {
				c.JSON(http.StatusConflict, gin.H{
					"status":   "duplicado",
					"mensagem": "eleitor já votou nesta eleição",
				})
				return
			}
			fmt.Printf("⚠️ Erro na validação (Redis): %v\n", err)
			// Em caso de erro interno, logamos mas permitimos o processamento (fail-open)
			// para não prejudicar o usuário por falhas de infraestrutura.
		}

		// Publicação no Pipeline
		if err := wp.Publicar(voto); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "erro",
				"mensagem": "sistema sobrecarregado, tente novamente",
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status":   "aceito",
			"mensagem": "voto recebido com sucesso",
		})
	})

	// Endpoint de Saúde de Produção: Verifica se as dependências fundamentais estão respondendo
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// 1. Verifica Redis
		if err := val.Checking(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "erro", "servico": "redis", "detalhe": err.Error()})
			return
		}

		// 2. Verifica se o WorkerPool está saudável
		if wp.EstaSobrecarregado() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "erro", "servico": "worker_pool", "detalhe": "vazão muito alta"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	return r
}
