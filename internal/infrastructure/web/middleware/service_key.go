package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"bitbucket.org/appmax-space/go-boilerplate/pkg/httputil"
	"bitbucket.org/appmax-space/go-boilerplate/pkg/logutil"
)

// ServiceKeyConfig contém a configuração de autenticação por Service Key.
type ServiceKeyConfig struct {
	// Enabled indica se a autenticação está habilitada.
	// Quando true e Keys está vazio, rejeita todas as requisições com 503 (fail-closed).
	// Quando false, permite todas as requisições (modo desenvolvimento).
	Enabled bool

	// Keys é um mapa de service-name → key autorizado.
	// Formato de entrada (env): "service1:key1,service2:key2"
	Keys map[string]string

	// ServiceNameHeader é o header que contém o nome do serviço chamador.
	// Default: "X-Service-Name"
	ServiceNameHeader string

	// ServiceKeyHeader é o header que contém a chave do serviço.
	// Default: "X-Service-Key"
	ServiceKeyHeader string
}

// ParseServiceKeys converte uma string no formato "service1:key1,service2:key2"
// para um mapa de chaves.
func ParseServiceKeys(raw string) map[string]string {
	keys := make(map[string]string)
	if raw == "" {
		return keys
	}

	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			serviceName := strings.TrimSpace(parts[0])
			serviceKey := strings.TrimSpace(parts[1])
			if serviceName != "" && serviceKey != "" {
				keys[serviceName] = serviceKey
			}
		}
	}
	return keys
}

// DefaultServiceKeyConfig retorna a configuração padrão.
func DefaultServiceKeyConfig() ServiceKeyConfig {
	return ServiceKeyConfig{
		Keys:              make(map[string]string),
		ServiceNameHeader: "X-Service-Name",
		ServiceKeyHeader:  "X-Service-Key",
	}
}

// ServiceKeyAuth retorna um middleware que valida a autenticação via Service Key.
//
// Comportamento baseado no campo Enabled:
//   - Enabled=false: permite todas as requisições (modo desenvolvimento)
//   - Enabled=true + Keys vazio: rejeita todas as requisições com 503 (fail-closed)
//   - Enabled=true + Keys populado: valida normalmente
func ServiceKeyAuth(config ServiceKeyConfig) gin.HandlerFunc {
	// Define headers padrão se não configurados
	if config.ServiceNameHeader == "" {
		config.ServiceNameHeader = "X-Service-Name"
	}
	if config.ServiceKeyHeader == "" {
		config.ServiceKeyHeader = "X-Service-Key"
	}

	return func(c *gin.Context) {
		// Se auth não está habilitada, permite tudo (modo desenvolvimento)
		if !config.Enabled {
			c.Next()
			return
		}

		// Fail-closed: auth habilitada mas sem chaves configuradas.
		// Impede que um deploy sem SERVICE_KEYS em HML/PRD exponha o serviço.
		if len(config.Keys) == 0 {
			httputil.SendError(c, http.StatusServiceUnavailable, "service authentication not configured")
			c.Abort()
			return
		}

		serviceName := c.GetHeader(config.ServiceNameHeader)
		serviceKey := c.GetHeader(config.ServiceKeyHeader)

		// Validate headers present
		if serviceName == "" || serviceKey == "" {
			httputil.SendError(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		// Validar chave do serviço
		expectedKey, exists := config.Keys[serviceName]
		if !exists {
			httputil.SendError(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(expectedKey), []byte(serviceKey)) != 1 {
			httputil.SendError(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		// Enrich LogContext with caller service for downstream logging
		lc, _ := logutil.Extract(c.Request.Context())
		lc.CallerService = serviceName
		c.Request = c.Request.WithContext(logutil.Inject(c.Request.Context(), lc))

		c.Next()
	}
}
