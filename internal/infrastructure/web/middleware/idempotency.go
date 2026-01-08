package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type IdempotencyConfig struct {
	TTL        time.Duration
	HeaderName string
}

// idempotencyEntry representa uma resposta cacheada
type idempotencyEntry struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	ExpiresAt  time.Time
}

// TODO: implementar redis
// MemoryIdempotencyStore é uma implementação in-memory do store de idempotência
// NOTA: Em produção, usar Redis para suportar múltiplas instâncias
type MemoryIdempotencyStore struct {
	entries map[string]idempotencyEntry
	mu      sync.RWMutex
}

// NewMemoryIdempotencyStore cria um novo store in-memory
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	store := &MemoryIdempotencyStore{
		entries: make(map[string]idempotencyEntry),
	}

	// Goroutine para limpar entradas expiradas
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			store.cleanup()
		}
	}()

	return store
}

func (s *MemoryIdempotencyStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.entries {
		if entry.ExpiresAt.Before(now) {
			delete(s.entries, key)
		}
	}
}

func (s *MemoryIdempotencyStore) Get(key string) (idempotencyEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[key]
	if !exists || entry.ExpiresAt.Before(time.Now()) {
		return idempotencyEntry{}, false
	}
	return entry, true
}

func (s *MemoryIdempotencyStore) Set(key string, entry idempotencyEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = entry
}

// responseWriter é um wrapper para capturar a resposta
type responseWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// Idempotency retorna um middleware de idempotência
// O cliente deve enviar um header X-Idempotency-Key único para cada operação
func Idempotency(config IdempotencyConfig) gin.HandlerFunc {
	store := NewMemoryIdempotencyStore()

	if config.HeaderName == "" {
		config.HeaderName = "X-Idempotency-Key"
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}

	return func(c *gin.Context) {
		// Apenas aplicar a métodos que modificam estado
		if c.Request.Method != http.MethodPost &&
			c.Request.Method != http.MethodPut &&
			c.Request.Method != http.MethodPatch {
			c.Next()
			return
		}

		idempotencyKey := c.GetHeader(config.HeaderName)
		if idempotencyKey == "" {
			// Sem chave de idempotência, processar normalmente
			c.Next()
			return
		}

		// Criar hash da chave + path para evitar colisões
		hasher := sha256.New()
		hasher.Write([]byte(idempotencyKey + c.Request.URL.Path))
		key := hex.EncodeToString(hasher.Sum(nil))

		// Verificar se já existe resposta cacheada
		if entry, exists := store.Get(key); exists {
			// Retornar resposta cacheada
			for k, v := range entry.Headers {
				for _, val := range v {
					c.Header(k, val)
				}
			}
			c.Header("X-Idempotent-Replayed", "true")
			c.Data(entry.StatusCode, "application/json", entry.Body)
			c.Abort()
			return
		}

		// Capturar resposta
		rw := &responseWriter{ResponseWriter: c.Writer}
		c.Writer = rw

		c.Next()

		// Armazenar resposta se sucesso (2xx)
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			store.Set(key, idempotencyEntry{
				StatusCode: c.Writer.Status(),
				Body:       rw.body,
				Headers:    c.Writer.Header().Clone(),
				ExpiresAt:  time.Now().Add(config.TTL),
			})
		}
	}
}

// DefaultIdempotencyConfig retorna configuração padrão
func DefaultIdempotencyConfig() IdempotencyConfig {
	return IdempotencyConfig{
		TTL:        24 * time.Hour,
		HeaderName: "X-Idempotency-Key",
	}
}
