package dto

// =============================================================================
// Create Entity DTOs
// =============================================================================

// CreateInput representa os dados de entrada para criação de entity.
type CreateInput struct {
	Name  string `json:"name"`  // Nome da entity
	Email string `json:"email"` // Email (será validado no UseCase)
}

// CreateOutput representa os dados de saída após criação.
type CreateOutput struct {
	ID        string `json:"id"`         // ID gerado (ULID)
	CreatedAt string `json:"created_at"` // Timestamp no formato RFC3339
}
