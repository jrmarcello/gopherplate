package dto

// =============================================================================
// Get Entity DTOs
// =============================================================================

// GetInput representa os dados de entrada para buscar uma entity.
type GetInput struct {
	ID string `json:"id"` // ULID da entity
}

// GetOutput representa os dados de saída da entity encontrada.
type GetOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
