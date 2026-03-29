package dto

// =============================================================================
// Create User DTOs
// =============================================================================

// CreateInput representa os dados de entrada para criação de user.
type CreateInput struct {
	Name  string `json:"name" binding:"required,max=255"`        // Nome do user
	Email string `json:"email" binding:"required,email,max=255"` // Email (validado via binding + UseCase)
}

// CreateOutput representa os dados de saída após criação.
type CreateOutput struct {
	ID        string `json:"id"`         // ID gerado (UUID v7)
	CreatedAt string `json:"created_at"` // Timestamp no formato RFC3339
}
