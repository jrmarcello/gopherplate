package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiterConfig contém configurações do rate limiter
type RateLimiterConfig struct {
	// RequestsPerSecond é o número de requisições permitidas por segundo
	RequestsPerSecond float64
	// BurstSize é o tamanho máximo do burst
	BurstSize int
}

// IPRateLimiter gerencia rate limiters por IP
type IPRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   RateLimiterConfig
}

// NewIPRateLimiter cria um novo rate limiter por IP
func NewIPRateLimiter(config RateLimiterConfig) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   config,
	}
}

// getLimiter retorna o limiter para um IP específico
func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.limiters[ip]
	i.mu.RUnlock()

	if exists {
		return limiter
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	// Double check após adquirir lock exclusivo
	if limiter, exists = i.limiters[ip]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rate.Limit(i.config.RequestsPerSecond), i.config.BurstSize)
	i.limiters[ip] = limiter

	return limiter
}

// RateLimit retorna um middleware de rate limiting por IP
func RateLimit(config RateLimiterConfig) gin.HandlerFunc {
	limiter := NewIPRateLimiter(config)

	// Goroutine para limpar limiters antigos periodicamente
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			limiter.mu.Lock()
			limiter.limiters = make(map[string]*rate.Limiter)
			limiter.mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		l := limiter.getLimiter(ip)

		if !l.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": "1s",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// DefaultRateLimitConfig retorna configuração padrão de rate limiting
func DefaultRateLimitConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 10,
		BurstSize:         20,
	}
}
