package router

import (
	"net/http"

	"contagem/rabbitmq/consumer"
	"contagem/redis"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func Iniciar(rdb *redis.Client) *gin.Engine {
	r := gin.Default()

	// CORS Básico para Produção
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

	// 1. Snapshot Completo (Alto Desempenho - LÊ DA RAM)
	r.GET("/votos/all", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		// ⚡ LÊ DA RAM (0ms de latência)
		consumer.SnapshotMutex.RLock()
		snapshot := make(map[string]int64)
		for k, v := range consumer.SnapshotCache {
			snapshot[k] = v
		}
		consumer.SnapshotMutex.RUnlock()

		c.JSON(http.StatusOK, snapshot)
	})

	// 2. Votos por Emenda Específica (LÊ DA RAM)
	r.GET("/votos/emenda/:id", func(c *gin.Context) {
		id := c.Param("id")

		consumer.SnapshotMutex.RLock()
		votos := consumer.SnapshotCache[id]
		consumer.SnapshotMutex.RUnlock()

		c.JSON(http.StatusOK, gin.H{"id": id, "votos": votos})
	})

	// 3. WebSocket Real-time Hub
	r.GET("/ws/votos", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Envia snapshot imediato no connect (DA RAM)
		go func() {
			consumer.SnapshotMutex.RLock()
			snapshot := make(map[string]int64)
			for k, v := range consumer.SnapshotCache {
				snapshot[k] = v
			}
			consumer.SnapshotMutex.RUnlock()
			_ = conn.WriteJSON(snapshot)
		}()

		// Assina o canal de tempo real do Redis
		ch := rdb.Subscribe(c.Request.Context(), "votos_realtime")

		for msg := range ch {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				break
			}
		}
	})

	return r
}
