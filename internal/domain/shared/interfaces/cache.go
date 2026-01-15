package interfaces

import "context"

// Cache define o contrato para operações de cache genérico.
// Esta interface pode ser utilizada por qualquer domínio da aplicação.
type Cache interface {
	// Get recupera um valor do cache e deserializa em dest.
	// Retorna erro se a chave não existir ou se houver falha na deserialização.
	Get(ctx context.Context, key string, dest interface{}) error

	// Set armazena um valor no cache com TTL padrão.
	Set(ctx context.Context, key string, value interface{}) error

	// Delete remove uma chave do cache.
	Delete(ctx context.Context, key string) error

	// Ping verifica se a conexão com o cache está saudável.
	Ping(ctx context.Context) error
}
